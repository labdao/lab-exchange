package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
	"gorm.io/gorm"
)

func createCheckoutSession(walletAddress string) (*stripe.CheckoutSession, error) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(frontendURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String("price_1OehLu2mES9P7kjwSQS45ZKq"), // comes from Stripe Product Dashboard
				Quantity: stripe.Int64(1),
				AdjustableQuantity: &stripe.CheckoutSessionLineItemAdjustableQuantityParams{
					Enabled: stripe.Bool(true),
				},
			},
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"walletAddress": walletAddress,
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
	}
	params.AddMetadata("walletAddress", walletAddress)
	result, err := session.New(params)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func StripeCreateCheckoutSessionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		walletAddress := params["walletAddress"]

		session, err := createCheckoutSession(walletAddress)
		if err != nil {
			// Handle the error appropriately
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Send the session URL back to the client
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": session.URL})
	}
}

func StripeFullfillmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET_KEY")

		event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying webhook signature: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch event.Type {
		case "payment_intent.succeeded":
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing payment intent: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			fmt.Printf("PaymentIntent succeeded, Amount: %v, WalletAddress: %v\n", paymentIntent.Amount, paymentIntent.Metadata["walletAddress"])

		default:
			fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	}
}
