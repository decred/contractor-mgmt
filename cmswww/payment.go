package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pd "github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

type polledPayment struct {
	address     string // Payment address
	amount      uint64 // Expected tx amount required to satisfy payment
	txNotBefore int64  // Minimum timestamp for payment tx
	pollExpiry  int64  // After this time, the payment address will not be continuously polled
}

const (
	// pollExpiryDuration is the amount of time the server will watch a payment address
	// for transactions.
	pollExpiryDuration = time.Hour * 24

	// pollCheckGap is the amount of time the server sleeps after polling for
	// a payment address.
	pollCheckGap = time.Second * 5
)

func pollHasExpired(pollExpiry int64) bool {
	return time.Now().After(time.Unix(pollExpiry, 0))
}

func (c *cmswww) derivePaymentInfo(user *database.User) (string, int64, error) {
	address, err := util.DerivePaywallAddress(c.params,
		user.ExtendedPublicKey, uint32(user.PaymentAddressIndex))
	if err != nil {
		err = fmt.Errorf("Unable to derive payment address "+
			"for user %v (%v): %v", user.ID, user.Email, err)
		return "", 0, err
	}

	// Update the user in the database.
	user.PaymentAddressIndex++
	err = c.db.UpdateUser(user)
	return address, time.Now().Unix(), err
}

// _addInvoiceForPolling adds an invoice's payment info to the in-memory map.
//
// This function must be called WITH the mutex held.
func (c *cmswww) _addInvoiceForPolling(token string, invoicePayment *database.InvoicePayment) {
	c.polledPayments[token] = polledPayment{
		address:     invoicePayment.Address,
		amount:      invoicePayment.Amount,
		txNotBefore: invoicePayment.TxNotBefore,
		pollExpiry:  invoicePayment.PollExpiry,
	}
}

// addInvoiceForPolling adds an invoice's payment info to the in-memory map.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) addInvoiceForPolling(token string, invoicePayment *database.InvoicePayment) {
	c.Lock()
	defer c.Unlock()

	c._addInvoiceForPolling(token, invoicePayment)
}

func (c *cmswww) addInvoicesForPolling() error {
	c.Lock()
	defer c.Unlock()

	// Create the in-memory pool of all invoices that need to be paid.
	invoices, _, err := c.db.GetInvoices(database.InvoicesRequest{
		StatusMap: map[v1.InvoiceStatusT]bool{
			v1.InvoiceStatusApproved: true,
		},
		Page: -1,
	})
	if err != nil {
		return err
	}

	for _, inv := range invoices {
		invoice, err := c.db.GetInvoiceByToken(inv.Token)
		if err != nil {
			return err
		}

		for _, invoicePayment := range invoice.Payments {
			if invoicePayment.TxID != "" {
				continue
			}

			invoicePayment.PollExpiry =
				time.Now().Add(pollExpiryDuration).Unix()

			err = c.db.UpdateInvoicePayment(&invoicePayment)
			if err != nil {
				return err
			}

			c._addInvoiceForPolling(invoice.Token, &invoicePayment)
		}
	}

	log.Tracef("Added %v invoices to the payment pool", len(c.polledPayments))
	return nil
}

func (c *cmswww) createPolledPaymentsCopy() map[string]polledPayment {
	c.RLock()
	defer c.RUnlock()

	copy := make(map[string]polledPayment, len(c.polledPayments))

	for k, v := range c.polledPayments {
		copy[k] = v
	}

	return copy
}

func (c *cmswww) checkForInvoicePayments(polledPayments map[string]polledPayment) (bool, []string) {
	var tokensToRemove []string

	for token, polledPayment := range polledPayments {
		dbInvoice, err := c.db.GetInvoiceByToken(token)
		if err != nil {
			if err == database.ErrShutdown {
				// The database is shutdown, so stop the thread.
				return false, nil
			}

			log.Errorf("cannot fetch invoice by token %v: %v\n", token, err)
			continue
		}

		log.Tracef("Checking the payment address for invoice %v...",
			token)

		if dbInvoice.Status == v1.InvoiceStatusPaid {
			// The invoice could have been marked as paid by some external
			// mechanism, so just remove it from polling.
			tokensToRemove = append(tokensToRemove, token)
			log.Tracef("  removing from polling, invoice already paid")
			continue
		}

		if pollHasExpired(polledPayment.pollExpiry) {
			tokensToRemove = append(tokensToRemove, dbInvoice.Token)
			log.Tracef("  removing from polling, poll has expired")
			continue
		}

		tx, _, err := util.FetchTxWithBlockExplorers(polledPayment.address,
			polledPayment.amount, polledPayment.txNotBefore,
			c.cfg.MinConfirmationsRequired)
		if err != nil {
			log.Errorf("cannot fetch tx: %v\n", err)
			continue
		}

		if tx != "" {
			// Create the payment metadata record.
			mdPayment, err := json.Marshal(BackendInvoiceMDPayment{
				Version:     VersionBackendInvoiceMDPayment,
				Address:     polledPayment.address,
				Amount:      polledPayment.amount,
				TxNotBefore: polledPayment.txNotBefore,
				TxID:        tx,
			})
			if err != nil {
				log.Errorf("cannot marshal backend payment: %v", err)
				continue
			}

			challenge, err := util.Random(pd.ChallengeSize)
			if err != nil {
				log.Errorf("could not create challenge: %v", err)
				continue
			}

			pdCommand := pd.UpdateVettedMetadata{
				Challenge: hex.EncodeToString(challenge),
				Token:     dbInvoice.Token,
				MDOverwrite: []pd.MetadataStream{
					{
						ID:      mdStreamPayments,
						Payload: string(mdPayment),
					},
				},
			}

			responseBody, err := c.rpc(http.MethodPost,
				pd.UpdateVettedMetadataRoute, pdCommand)
			if err != nil {
				log.Errorf("problem communicating with politeiad: %v", err)
				continue
			}

			var pdReply pd.UpdateVettedMetadataReply
			err = json.Unmarshal(responseBody, &pdReply)
			if err != nil {
				log.Errorf("could not unmarshal UpdateVettedMetadataReply: %v",
					err)
				continue
			}

			// Verify the challenge.
			err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
			if err != nil {
				log.Errorf("could not verify challenge: %v",
					err)
				continue
			}

			// Update the invoice in the database.
			dbInvoice.Status = v1.InvoiceStatusPaid
			for idx, invoicePayment := range dbInvoice.Payments {
				if invoicePayment.Address == polledPayment.address &&
					invoicePayment.Amount == polledPayment.amount &&
					invoicePayment.TxNotBefore == polledPayment.txNotBefore {
					dbInvoice.Payments[idx].TxID = tx
				}
			}
			dbInvoice.Changes = append(dbInvoice.Changes, database.InvoiceChange{
				Timestamp: time.Now().Unix(),
				NewStatus: v1.InvoiceStatusPaid,
			})
			err = c.db.UpdateInvoice(dbInvoice)
			if err != nil {
				if err == database.ErrShutdown {
					// The database is shutdown, so stop the thread.
					return false, nil
				}

				log.Errorf("cannot update invoice with token %v: %v",
					dbInvoice.Token, err)
				continue
			}

			c.fireEvent(EventTypeInvoicePaid,
				EventDataInvoicePaid{
					Invoice: dbInvoice,
					TxID:    tx,
				},
			)

			// Remove this invoice from polling.
			tokensToRemove = append(tokensToRemove, token)
			log.Tracef("  removing from polling, invoice just paid")
		}

		time.Sleep(pollCheckGap)
	}

	return true, tokensToRemove
}

func (c *cmswww) removeInvoicesFromPolling(tokensToRemove []string) {
	c.Lock()
	defer c.Unlock()

	for _, token := range tokensToRemove {
		delete(c.polledPayments, token)
	}
}

func (c *cmswww) checkForPayments() {
	for {
		invoicePaymentsToCheck := c.createPolledPaymentsCopy()
		shouldContinue, invoiceTokensToRemove := c.checkForInvoicePayments(invoicePaymentsToCheck)
		if !shouldContinue {
			return
		}
		c.removeInvoicesFromPolling(invoiceTokensToRemove)

		time.Sleep(pollCheckGap)
	}
}

func (c *cmswww) initPaymentChecker() error {
	err := c.addInvoicesForPolling()
	if err != nil {
		return err
	}

	// Start the thread that checks for payments.
	go c.checkForPayments()
	return nil
}
