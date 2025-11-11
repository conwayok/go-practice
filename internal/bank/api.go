package bank

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type API struct {
	logger  *slog.Logger
	service Service
}

func NewAPI(logger *slog.Logger, service Service) *API {
	return &API{
		logger:  logger,
		service: service,
	}
}

func (a *API) RegisterRoutes(router chi.Router) {
	router.Route("/api/currencies", func(r chi.Router) {
		r.Get("/", a.getCurrencies)
		r.Post("/", a.createCurrency)
	})
	router.Route("/api/accounts", func(r chi.Router) {
		r.Post("/", a.createAccount)
		r.Get("/{id}/info", a.getAccountInfo)
	})
	router.Post("/api/deposits", a.deposit)
	router.Post("/api/withdrawals", a.withdraw)
	router.Post("/api/transfers", a.transfer)
}

func (a *API) getCurrencies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.GetCurrencies(ctx)

	if err != nil {
		a.logger.Error("get currencies failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func (a *API) createCurrency(w http.ResponseWriter, r *http.Request) {
	var request CreateCurrencyRequest
	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		writeBadRequestInvalidJSON(w)
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	err = a.service.CreateCurrency(ctx, request)

	if err != nil {

		if errors.Is(err, ErrInvalidCurrencyID) || errors.Is(err, ErrInvalidCurrencyDecimals) {
			writeErrorResponse(w, ErrorCodeInvalidParams, err.Error())
			return
		}

		if errors.Is(err, ErrCurrencyAlreadyExists) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		a.logger.Error("create currencies failed", "error", err)
		writeUnknownError(w)
		return
	}

	w.WriteHeader(200)
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	var request CreateAccountRequest
	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		writeBadRequestInvalidJSON(w)
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.CreateAccount(ctx, request)

	if err != nil {

		if errors.Is(err, ErrAccountNameInvalid) {
			writeErrorResponse(w, ErrorCodeInvalidParams, err.Error())
			return
		}

		if errors.Is(err, ErrCurrencyNotFound) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		if errors.Is(err, ErrIdempotencyKeyExists) {
			writeErrorResponse(w, ErrorCodeDuplicateIdempotencyKey, err.Error())
			return
		}

		a.logger.Error("create account failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func (a *API) getAccountInfo(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "id")

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)

	if err != nil {
		writeErrorResponse(w, ErrorCodeInvalidParams, "invalid account ID")
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.GetAccountInfo(ctx, GetAccountInfoRequest{
		ID: accountID,
	})

	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		a.logger.Error("get account info failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func (a *API) deposit(w http.ResponseWriter, r *http.Request) {
	var request DepositRequest
	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		writeBadRequestInvalidJSON(w)
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.Deposit(ctx, request)

	if err != nil {

		if errors.Is(err, ErrInvalidTransactionAmount) {
			writeErrorResponse(w, ErrorCodeInvalidParams, err.Error())
			return
		}

		if errors.Is(err, ErrIdempotencyKeyExists) {
			writeErrorResponse(w, ErrorCodeDuplicateIdempotencyKey, err.Error())
			return
		}

		if errors.Is(err, ErrCurrencyNotFound) || errors.Is(err, ErrAccountNotFound) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		a.logger.Error("deposit failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func (a *API) withdraw(w http.ResponseWriter, r *http.Request) {
	var request WithdrawRequest
	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		writeBadRequestInvalidJSON(w)
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.Withdraw(ctx, request)

	if err != nil {

		if errors.Is(err, ErrInvalidTransactionAmount) {
			writeErrorResponse(w, ErrorCodeInvalidParams, err.Error())
			return
		}

		if errors.Is(err, ErrIdempotencyKeyExists) {
			writeErrorResponse(w, ErrorCodeDuplicateIdempotencyKey, err.Error())
			return
		}

		if errors.Is(err, ErrCurrencyNotFound) || errors.Is(err, ErrAccountNotFound) || errors.Is(err, ErrInsufficientBalance) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		a.logger.Error("withdraw failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func (a *API) transfer(w http.ResponseWriter, r *http.Request) {
	var request TransferRequest
	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		writeBadRequestInvalidJSON(w)
		return
	}

	ctx, cancel := getDefaultContext(r.Context())
	defer cancel()

	response, err := a.service.Transfer(ctx, request)

	if err != nil {

		if errors.Is(err, ErrInvalidTransactionAmount) {
			writeErrorResponse(w, ErrorCodeInvalidParams, err.Error())
			return
		}

		if errors.Is(err, ErrIdempotencyKeyExists) {
			writeErrorResponse(w, ErrorCodeDuplicateIdempotencyKey, err.Error())
			return
		}

		if errors.Is(err, ErrTransferFromAccountNotFound) || errors.Is(err, ErrTransferToAccountNotFound) || errors.Is(err, ErrTransferCurrencyMismatch) || errors.Is(err, ErrInsufficientBalance) {
			writeErrorResponse(w, ErrorCodeInvalidOperation, err.Error())
			return
		}

		a.logger.Error("withdraw failed", "error", err)
		writeUnknownError(w)
		return
	}

	writeJSON(w, 200, response)
}

func getDefaultContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Second)
}

func writeUnknownError(w http.ResponseWriter) {
	writeErrorResponse(w, ErrorCodeUnknown, "unknown error")
}

func writeBadRequestInvalidJSON(w http.ResponseWriter) {
	writeErrorResponse(w, ErrorCodeInvalidParams, "invalid JSON")
}

func writeErrorResponse(w http.ResponseWriter, code ErrorCode, message string) {

	var status int

	if code == ErrorCodeUnknown {
		status = 500
	} else {
		status = 400
	}

	body := APIErrorResponse{
		Code:    code,
		Message: message,
	}

	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}
