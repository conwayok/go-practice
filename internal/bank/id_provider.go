package bank

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

type IDProvider interface {
	NextID() int64
}

type idProvider struct {
	snowflakeNode *snowflake.Node
}

func NewIDProvider(nodeID int64) (IDProvider, error) {

	node, err := snowflake.NewNode(nodeID)

	if err != nil {
		return nil, fmt.Errorf("init snowflake node failed: %w", err)
	}

	return &idProvider{
		snowflakeNode: node,
	}, nil
}

func (i *idProvider) NextID() int64 {
	return i.snowflakeNode.Generate().Int64()
}
