package handler

import (
	"backer/transaction"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	transactionIndexTemplate = "transaction_index.html"
)

type transactionHandler struct {
	transactionService transaction.Service
}

func NewTransactionHandler(transactionService transaction.Service) *transactionHandler {
	return &transactionHandler{transactionService}
}

func (h *transactionHandler) Index(c *gin.Context) {
	transactions, err := h.transactionService.GetAllTransactions()
	if err != nil {
		c.HTML(http.StatusInternalServerError, errorTemplate, nil)
		return
	}

	c.HTML(http.StatusOK, transactionIndexTemplate, gin.H{"transactions": transactions})
}
