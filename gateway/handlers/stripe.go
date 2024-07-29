package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/labdao/plex/gateway/middleware"
	"github.com/labdao/plex/gateway/models"
	"github.com/labdao/plex/gateway/utils"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/subscription"
	"github.com/stripe/stripe-go/v78/webhook"
	"gorm.io/gorm"
)

func setupStripeClient() error {
	apiKey := os.Getenv("STRIPE_SECRET_KEY")
	if apiKey == "" {
		return errors.New("STRIPE_SECRET_KEY environment variable not set")
	}
	stripe.Key = apiKey
	return nil
}

func createStripeCustomer(walletAddress string) (string, error) {
	err := setupStripeClient()
	if err != nil {
		return "", err
	}

	params := &stripe.CustomerParams{
		Name: stripe.String(walletAddress),
	}
	customer, err := customer.New(params)
	if err != nil {
		return "", err
	}

	return customer.ID, nil
}

func createCheckoutSession(stripeUserID, walletAddress string) (*stripe.CheckoutSession, error) {
	err := setupStripeClient()
	if err != nil {
		return nil, err
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	priceID := os.Getenv("STRIPE_PRICE_ID")
	if priceID == "" {
		return nil, errors.New("STRIPE_PRICE_ID environment variable not set")
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeUserID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price: stripe.String(priceID),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(7),
			Metadata: map[string]string{
				"Wallet Address": walletAddress,
			},
		},
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		SuccessURL:         stripe.String(frontendURL + "/checkout/success"),
		CancelURL:          stripe.String(frontendURL + "/checkout/cancel"),
		Metadata: map[string]string{
			"Wallet Address": walletAddress,
		},
	}

	session, err := session.New(params)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func cancelStripeSubscription(subscriptionID string) error {
	err := setupStripeClient()
	if err != nil {
		return err
	}

	_, err = subscription.Cancel(subscriptionID, nil)
	return err
}

func StripeCreateCheckoutSessionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctxUser := r.Context().Value(middleware.UserContextKey)
		user, ok := ctxUser.(*models.User)
		if !ok {
			utils.SendJSONError(w, "Unauthorized, user context not passed through auth middleware", http.StatusUnauthorized)
			return
		}

		thresholdStr := os.Getenv("TIER_THRESHOLD")
		threshold, _ := strconv.Atoi(thresholdStr)

		if user.ComputeTally < threshold || user.SubscriptionStatus == "active" {
			utils.SendJSONError(w, "User does not need a subscription at this time", http.StatusBadRequest)
			return
		}

		session, err := createCheckoutSession(user.StripeUserID, user.WalletAddress)
		if err != nil {
			utils.SendJSONError(w, fmt.Sprintf("Error creating checkout session: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Println("Checkout URL:", session.URL)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": session.URL})
	}
}

func StripeFulfillmentHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET_KEY")

		event, err := webhook.ConstructEventWithOptions(payload, r.Header.Get("Stripe-Signature"), endpointSecret, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying webhook signature: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch event.Type {
		case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing subscription: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			walletAddress, ok := subscription.Metadata["Wallet Address"]
			if !ok {
				fmt.Fprintf(os.Stderr, "Wallet Address not found in subscription metadata\n")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var user models.User
			result := db.Where("wallet_address ILIKE ?", walletAddress).First(&user)
			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrRecordNotFound) {
					fmt.Fprintf(os.Stderr, "User with wallet address %s not found\n", walletAddress)
					w.WriteHeader(http.StatusNotFound)
				} else {
					fmt.Fprintf(os.Stderr, "Error querying user: %v\n", result.Error)
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}

			if subscription.Status == "trialing" || subscription.Status == "active" {
				user.SubscriptionStatus = "active"
			} else {
				user.SubscriptionStatus = string(subscription.Status)
			}
			user.SubscriptionID = &subscription.ID
			result = db.Save(&user)
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error updating user subscription status: %v\n", result.Error)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			fmt.Printf("Subscription %s for user %s updated to %s\n", subscription.ID, walletAddress, user.SubscriptionStatus)

		case "customer.subscription.trial_will_end":
			// Handle trial ending soon (e.g., send notification to user)
			// This event occurs 3 days before the trial ends
			// You might want to implement logic to notify the user

		case "invoice.paid", "invoice.payment_failed":
			// Handle successful or failed payments
			// You might want to update the user's payment status or send notifications

		default:
			fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func StripeCancelSubscriptionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctxUser := r.Context().Value(middleware.UserContextKey)
		user, ok := ctxUser.(*models.User)
		if !ok {
			utils.SendJSONError(w, "Unauthorized, user context not passed through auth middleware", http.StatusUnauthorized)
			return
		}

		if user.SubscriptionID == nil {
			utils.SendJSONError(w, "User does not have an active subscription", http.StatusBadRequest)
			return
		}

		err := cancelStripeSubscription(*user.SubscriptionID)
		if err != nil {
			utils.SendJSONError(w, fmt.Sprintf("Error canceling subscription: %v", err), http.StatusInternalServerError)
			return
		}

		user.SubscriptionStatus = "canceled"
		user.SubscriptionID = nil
		result := db.Save(&user)
		if result.Error != nil {
			utils.SendJSONError(w, fmt.Sprintf("Error updating user subscription status: %v", result.Error), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Subscription canceled successfully"})
	}
}
