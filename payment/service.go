package payment

import (
	"backer/user"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"strconv"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

type service struct {
	serverKey string
	env       midtrans.EnvironmentType
}

type Service interface {
	GetPaymentURL(transaction Transaction, user user.User) (string, error)
	VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool
}

type Transaction struct {
	ID     int
	Amount int
}

func (s *service) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	payload := orderID + statusCode + grossAmount + s.serverKey
	hash := sha512.Sum512([]byte(payload))
	expectedSignature := hex.EncodeToString(hash[:])
	return expectedSignature == signatureKey
}

func NewService() *service {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	env := midtrans.Sandbox

	if os.Getenv("MIDTRANS_ENV") == "production" {
		env = midtrans.Production
	}

	return &service{
		serverKey: serverKey,
		env:       env,
	}
}

func (s *service) GetPaymentURL(transaction Transaction, user user.User) (string, error) {
	var snapClient snap.Client
	snapClient.New(s.serverKey, s.env)

	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  strconv.Itoa(transaction.ID),
			GrossAmt: int64(transaction.Amount),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: user.Name,
			Email: user.Email,
		},
		Callbacks: &snap.Callbacks{
			Finish: os.Getenv("FRONTEND_URL") + "/transaction/finish",
		},
	}

	snapResp, err := snapClient.CreateTransaction(snapReq)
	if err != nil {
		return "", err
	}
	return snapResp.RedirectURL, nil
}
