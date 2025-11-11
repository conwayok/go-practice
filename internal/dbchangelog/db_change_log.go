package dbchangelog

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type DbAuditLogQuery struct {
	TableNameIn  []string
	ChangedAtGte *time.Time
	ChangedAtLte *time.Time
	OrderDesc    bool
}

type Entry struct {
	Id        int64            `db:"id"`
	TableName string           `db:"table_name"`
	Operation string           `db:"operation"`
	Pk        *json.RawMessage `db:"pk"`
	RowData   *json.RawMessage `db:"row_data"`
	Diffs     *json.RawMessage `db:"diffs"`
	TxId      *int64           `db:"txid"`
	ChangedAt time.Time        `db:"changed_at"`
}

type Manager struct {
	tablePrefix string
}

func (d *Manager) getChangeLogTableName() string {
	return d.tablePrefix + "_change_logs"
}

func New(tablePrefix string) *Manager {
	return &Manager{
		tablePrefix: tablePrefix,
	}
}

var createTableSQL = `
CREATE TABLE IF NOT EXISTS __CHANGE_LOG_TABLE
(
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    TABLE_NAME   TEXT NOT NULL,
    operation    TEXT NOT NULL,
    pk           JSONB,
    row_data     JSONB,
    diffs        JSONB,
    txid         BIGINT,
    changed_at   TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx___CHANGE_LOG_TABLE_table_name ON __CHANGE_LOG_TABLE (TABLE_NAME);
CREATE INDEX IF NOT EXISTS idx___CHANGE_LOG_TABLE_logs_changed_at ON __CHANGE_LOG_TABLE (changed_at);

`

var createTriggerFuncSQL = `
CREATE OR REPLACE FUNCTION __CHANGE_LOG_TABLE_trigger()
    RETURNS TRIGGER
    LANGUAGE plpgsql
AS
$$
DECLARE
    diffs   JSONB := '{}'::jsonb;
    key     TEXT;
    pk_cols TEXT[];
    pk      JSONB := '{}'::jsonb;
    src     JSONB;
BEGIN
    -- Pick row snapshot
    src := CASE WHEN TG_OP = 'DELETE' THEN TO_JSONB(OLD) ELSE TO_JSONB(NEW) END;

    -- Detect PK columns for this table
    SELECT ARRAY_AGG(a.attname ORDER BY a.attnum)
    INTO pk_cols
    FROM pg_index i
             JOIN pg_attribute a
                  ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
    WHERE i.indrelid = TG_RELID
      AND i.indisprimary;

    -- Build pk jsonb
    IF pk_cols IS NOT NULL THEN
        FOREACH key IN ARRAY pk_cols
            LOOP
                pk := pk || JSONB_BUILD_OBJECT(key, src -> key);
            END LOOP;
    END IF;

    -- Build diffs on UPDATE
    IF TG_OP = 'UPDATE' THEN
        FOR key IN SELECT JSONB_OBJECT_KEYS(TO_JSONB(NEW))
            LOOP
                IF (TO_JSONB(NEW) -> key) IS DISTINCT FROM (TO_JSONB(OLD) -> key) THEN
                    diffs := diffs || JSONB_BUILD_OBJECT(
                            key, JSONB_BUILD_OBJECT(
                                    'old', TO_JSONB(OLD) -> key,
                                    'new', TO_JSONB(NEW) -> key
                                 )
                                      );
                END IF;
            END LOOP;
    END IF;

    INSERT INTO __CHANGE_LOG_TABLE(TABLE_NAME,
                                    operation,
                                    pk,
                                    row_data,
                                    diffs,
                                    txid,
                                    changed_at)
    VALUES (TG_TABLE_NAME,
            TG_OP,
            pk,
            src,
            CASE WHEN TG_OP = 'UPDATE' THEN diffs ELSE NULL END,
            TXID_CURRENT(),
            CLOCK_TIMESTAMP());

    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;
`

var createTriggersSQL = `
DO
$$
    DECLARE
        tbl          RECORD;
        skip_tables  TEXT[] := ARRAY [ '__CHANGE_LOG_TABLE' ];
        trigger_name TEXT;
    BEGIN
        FOR tbl IN SELECT table_name FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA()
            LOOP
                IF NOT (tbl.table_name = ANY (skip_tables)) THEN
                    trigger_name := '__CHANGE_LOG_TABLE' || '_' || tbl.table_name;
                    EXECUTE FORMAT('DROP TRIGGER IF EXISTS %I ON %I', trigger_name, tbl.table_name);
                    EXECUTE FORMAT(
                            'CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION __CHANGE_LOG_TABLE_trigger()',
                            trigger_name, tbl.table_name);
                END IF;
            END LOOP;
    END
$$;
`

func (d *Manager) Setup(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)

	if err != nil {
		return fmt.Errorf("connect to DB failed: %w", err)
	}

	defer conn.Close(ctx)

	changeLogTableName := d.getChangeLogTableName()

	sql := strings.ReplaceAll(createTableSQL, "__CHANGE_LOG_TABLE", changeLogTableName)

	_, err = conn.Exec(ctx, sql)

	if err != nil {
		return fmt.Errorf("db change log create table failed: %w", err)
	}

	sql = strings.ReplaceAll(createTriggerFuncSQL, "__CHANGE_LOG_TABLE", changeLogTableName)

	_, err = conn.Exec(ctx, sql)

	if err != nil {
		return fmt.Errorf("db change log create trigger func failed: %w", err)
	}

	sql = strings.ReplaceAll(createTriggersSQL, "__CHANGE_LOG_TABLE", changeLogTableName)

	_, err = conn.Exec(ctx, sql)

	if err != nil {
		return fmt.Errorf("db change log create triggers failed: %w", err)
	}

	return err
}

func (d *Manager) GetLogs(ctx context.Context, conn *pgx.Conn, tableNamesIn []string) ([]Entry, error) {
	sql := "SELECT id, table_name, operation, pk, row_data, diffs, txid, changed_at FROM " + d.getChangeLogTableName() + " WHERE table_name = ANY($1)"

	rows, err := conn.Query(ctx, sql, tableNamesIn)

	if err != nil {
		return nil, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[Entry])

	return results, nil
}

func (d *Manager) ToAsciiTreeString(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ChangedAt.Before(entries[j].ChangedAt)
	})

	grouped := map[string][]Entry{}
	for _, e := range entries {
		key := "Tx <none>"
		if e.TxId != nil {
			key = fmt.Sprintf("Tx %d", *e.TxId)
		}
		grouped[key] = append(grouped[key], e)
	}

	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, key := range keys {
		group := grouped[key]
		first := group[0]
		buf.WriteString(fmt.Sprintf("%s @ %s\n", key, first.ChangedAt.Format(time.RFC3339)))
		for _, e := range group {
			renderEntry(&buf, e, " ├── ", " │    ")
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

func renderEntry(buf *bytes.Buffer, e Entry, branch, indent string) {
	buf.WriteString(fmt.Sprintf("%s%s\n", branch, strings.ToUpper(e.Operation)))
	buf.WriteString(fmt.Sprintf("%s└── %s\n", indent, e.TableName))

	if (strings.EqualFold(e.Operation, "UPDATE") || strings.EqualFold(e.Operation, "DELETE")) && e.Pk != nil {
		var pkMap map[string]any
		if err := json.Unmarshal(*e.Pk, &pkMap); err == nil {
			var parts []string
			for k, v := range pkMap {
				b, err := json.Marshal(v)
				valStr := ""
				if err != nil {
					valStr = fmt.Sprintf("%v", v)
				} else {
					valStr = string(b)
				}
				parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))
			}
			buf.WriteString(fmt.Sprintf("%s     └── [PK] : %s\n", indent, strings.Join(parts, ", ")))
		}
	}

	switch strings.ToUpper(e.Operation) {
	case "INSERT", "DELETE":
		renderRowData(buf, e.RowData, indent+"     ")
	case "UPDATE":
		renderDiffs(buf, e.Diffs, indent+"     ")
	default:
		buf.WriteString(fmt.Sprintf("%s     (unknown op)\n", indent))
	}
}
func renderRowData(buf *bytes.Buffer, data *json.RawMessage, indent string) {
	if data == nil {
		buf.WriteString(fmt.Sprintf("%s(no row data)\n", indent))
		return
	}
	dec := json.NewDecoder(bytes.NewReader(*data))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		buf.WriteString(fmt.Sprintf("%s(invalid row data)\n", indent))
		return
	}

	for k, v := range obj {
		switch n := v.(type) {
		case json.Number:
			buf.WriteString(fmt.Sprintf("%s├── %s : %s\n", indent, k, n.String()))
		default:
			buf.WriteString(fmt.Sprintf("%s├── %s : %v\n", indent, k, v))
		}
	}
}
func renderDiffs(buf *bytes.Buffer, data *json.RawMessage, indent string) {
	if data == nil {
		buf.WriteString(fmt.Sprintf("%s(no diffs)\n", indent))
		return
	}
	var obj map[string]map[string]any
	if err := json.Unmarshal(*data, &obj); err != nil {
		buf.WriteString(fmt.Sprintf("%s(invalid diffs)\n", indent))
		return
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		elbow := "├──"
		if i == len(keys)-1 {
			elbow = "└──"
		}
		diff := obj[k]

		formatValue := func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(b)
		}

		oldVal := formatValue(diff["old"])
		newVal := formatValue(diff["new"])
		buf.WriteString(fmt.Sprintf("%s%s %s : %s -> %s\n", indent, elbow, k, oldVal, newVal))
	}
}
