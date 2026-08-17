package transaction

import (
	"backer/campaign"
	"backer/payment"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Custom errors
var (
	ErrCampaignNotFound    = errors.New("campaign not found")
	ErrNotAuthorized       = errors.New("not authorized")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidOrderID      = errors.New("invalid order id")
)

type service struct {
	repository         Repository
	campaignRepository campaign.Repository
	paymentService     payment.Service
}

type Service interface {
	GetTransactionsByCampaignID(input GetCampaignTransactionsInput) ([]Transaction, error)
	GetTransactionsByUserID(userID int) ([]Transaction, error)
	CreateTransaction(input CreateTransactionInput) (Transaction, error)
	ProcessPayment(input TransactionNotificationInput) error
	GetAllTransactions() ([]Transaction, error)
}

func NewService(repository Repository, campaignRepository campaign.Repository, paymentService payment.Service) *service {
	return &service{repository, campaignRepository, paymentService}
}

func (s *service) GetTransactionsByCampaignID(input GetCampaignTransactionsInput) ([]Transaction, error) {
	campaign, err := s.campaignRepository.FindByID(input.ID)
	if err != nil {
		return []Transaction{}, err
	}

	if campaign.ID == 0 {
		return []Transaction{}, ErrCampaignNotFound
	}

	if campaign.UserID != input.User.ID {
		return []Transaction{}, ErrNotAuthorized
	}

	transactions, err := s.repository.GetByCampaignID(input.ID)
	if err != nil {
		return transactions, err
	}

	return transactions, nil
}

func (s *service) GetTransactionsByUserID(userID int) ([]Transaction, error) {
	transactions, err := s.repository.GetByUserID(userID)
	if err != nil {
		return transactions, err
	}

	return transactions, nil
}

func (s *service) CreateTransaction(input CreateTransactionInput) (Transaction, error) {
	campaign, err := s.campaignRepository.FindByID(input.CampaignID)
	if err != nil {
		return Transaction{}, err
	}

	if campaign.ID == 0 {
		return Transaction{}, ErrCampaignNotFound
	}

	transaction := Transaction{}
	transaction.CampaignID = input.CampaignID
	transaction.Amount = input.Amount
	transaction.UserID = input.User.ID
	transaction.Status = "pending"

	timestamp := time.Now().Format("20060102150405")
	transaction.Code = fmt.Sprintf("TRX-%s-%d-%d", timestamp, input.User.ID, input.CampaignID)

	newTransaction, err := s.repository.Save(transaction)
	if err != nil {
		return newTransaction, err
	}

	paymentTransaction := payment.Transaction{
		ID:     newTransaction.ID,
		Amount: newTransaction.Amount,
	}

	paymentURL, err := s.paymentService.GetPaymentURL(paymentTransaction, input.User)
	if err != nil {
		return newTransaction, err
	}

	newTransaction.PaymentURL = paymentURL
	newTransaction, err = s.repository.Update(newTransaction)
	if err != nil {
		return newTransaction, err
	}

	return newTransaction, nil
}

func (s *service) ProcessPayment(input TransactionNotificationInput) error {
	transactionID, err := strconv.Atoi(input.OrderID)
	if err != nil {
		return fmt.Errorf("%w: order_id=%q", ErrInvalidOrderID, input.OrderID)
	}

	transaction, err := s.repository.GetByID(transactionID)
	if err != nil {
		return err
	}

	if transaction.ID == 0 {
		return ErrTransactionNotFound
	}

	if !s.paymentService.VerifySignature(input.OrderID, input.StatusCode, input.GrossAmount, input.SignatureKey) {
		return ErrInvalidSignature
	}

	if transaction.Status == "paid" || transaction.Status == "cancelled" {
		return nil
	}

	switch input.TransactionStatus {
	case "capture":
		if input.FraudStatus == "accept" {
			transaction.Status = "paid"
		}
	case "settlement":
		transaction.Status = "paid"
	case "deny", "expire", "cancel":
		transaction.Status = "cancelled"
	}

	updatedTransaction, err := s.repository.Update(transaction)
	if err != nil {
		return err
	}

	if updatedTransaction.Status == "paid" {
		if err := s.applyCampaignProgress(updatedTransaction); err != nil {
			return err
		}
	}

	return nil
}

// applyCampaignProgress separates the campaign update logic so that
// ProcessPayment stays simpler (reducing the cognitive complexity flagged by SonarQube).
func (s *service) applyCampaignProgress(updatedTransaction Transaction) error {
	campaign, err := s.campaignRepository.FindByID(updatedTransaction.CampaignID)
	if err != nil {
		return err
	}

	campaign.BackerCount = campaign.BackerCount + 1
	campaign.CurrentAmount = campaign.CurrentAmount + updatedTransaction.Amount

	_, err = s.campaignRepository.Update(campaign)
	return err
}

func (s *service) GetAllTransactions() ([]Transaction, error) {
	transactions, err := s.repository.FindAll()
	if err != nil {
		return transactions, err
	}
	return transactions, nil
}
