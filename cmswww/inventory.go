package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"

	v1 "github.com/decred/contractor-mgmt/cmswww/api/v1"
	pd "github.com/decred/politeia/politeiad/api/v1"
)

var (
	errRecordNotFound = fmt.Errorf("record not found")
)

type inventoryRecord struct {
	record    pd.Record              // actual record
	invoiceMD BackendInvoiceMetadata // invoice metadata
	changes   []MDStreamChanges      // changes metadata
}

// invoicesRequest is used for passing parameters into the
// getInvoices() function.
type invoicesRequest struct {
	After     string
	Before    string
	UserID    string
	Month     uint16
	Year      uint16
	StatusMap map[v1.InvoiceStatusT]bool
}

// updateInventoryRecord updates an existing record.
//
// This function must be called WITH the mutex held.
func (c *cmswww) updateInventoryRecord(record pd.Record) {
	c.inventory[record.CensorshipRecord.Token] = &inventoryRecord{
		record: record,
	}
}

// newInventoryRecord adds a record to the inventory.
//
// This function must be called WITH the mutex held.
func (c *cmswww) newInventoryRecord(record pd.Record) error {
	t := record.CensorshipRecord.Token
	if _, ok := c.inventory[t]; ok {
		return fmt.Errorf("duplicate token: %v", t)
	}

	c.updateInventoryRecord(record)
	return nil
}

// loadInvoiceMD decodes backend invoice metadata and stores it inventory object.
//
// This function must be called WITH the mutex held.
func (c *cmswww) loadInvoiceMD(token, payload string) error {
	f := strings.NewReader(payload)
	d := json.NewDecoder(f)
	var md BackendInvoiceMetadata
	if err := d.Decode(&md); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}

	c.inventory[token].invoiceMD = md
	return nil
}

// loadChanges decodes chnages metadata and stores it inventory object.
//
// This function must be called WITH the mutex held.
func (c *cmswww) loadChanges(token, payload string) error {
	f := strings.NewReader(payload)
	d := json.NewDecoder(f)
	for {
		var md MDStreamChanges
		if err := d.Decode(&md); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		p := c.inventory[token]
		p.changes = append(p.changes, md)
	}
}

// loadFullRecord load an entire record into inventory.
//
// This function must be called WITH the mutex held.
func (c *cmswww) loadFullRecord(v pd.Record) {
	t := v.CensorshipRecord.Token

	// Fish metadata out as well
	var err error
	for _, m := range v.Metadata {
		switch m.ID {
		case mdStreamGeneral:
			err = c.loadInvoiceMD(t, m.Payload)
			if err != nil {
				log.Errorf("initializeInventory "+
					"could not load metadata: %v",
					err)
				continue
			}
		case mdStreamChanges:
			err = c.loadChanges(t, m.Payload)
			if err != nil {
				log.Errorf("initializeInventory "+
					"could not load changes: %v",
					err)
				continue
			}
		default:
			// log error but proceed
			log.Errorf("initializeInventory: invalid "+
				"metadata stream ID %v token %v",
				m.ID, t)
		}
	}
}

// initializeInventory initializes the inventory map and loads it with a
// InventoryReply.
//
// This function must be called WITH the mutex held.
func (c *cmswww) initializeInventory(inv *pd.InventoryReply) error {
	c.inventory = make(map[string]*inventoryRecord)

	for _, v := range append(inv.Vetted, inv.Branches...) {
		err := c.newInventoryRecord(v)
		if err != nil {
			return err
		}
		c.loadFullRecord(v)
	}

	return nil
}

// _getInventoryRecord reads an inventory record from the inventory cache.
//
// This function must be called WITH the mutex held.
func (c *cmswww) _getInventoryRecord(token string) (inventoryRecord, error) {
	r, ok := c.inventory[token]
	if !ok {
		return inventoryRecord{}, errRecordNotFound
	}
	return *r, nil
}

// getInventoryRecord returns an inventory record from the inventory cache.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) getInventoryRecord(token string) (inventoryRecord, error) {
	c.RLock()
	defer c.RUnlock()
	return c._getInventoryRecord(token)
}

// getInvoice returns a single invoice by its token
func (c *cmswww) getInvoice(token string) (v1.InvoiceRecord, error) {
	ir, err := c._getInventoryRecord(token)
	if err != nil {
		return v1.InvoiceRecord{}, err
	}
	pr := convertInvoiceFromInventoryRecord(&ir, c.userPubkeys)
	return pr, nil
}

// getInvoices returns a list of invoices that adheres to the requirements
// specified in the provided request.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) getInvoices(pr invoicesRequest) []v1.InvoiceRecord {
	c.RLock()

	allInvoices := make([]v1.InvoiceRecord, 0, len(c.inventory))
	for _, vv := range c.inventory {
		v := convertInvoiceFromInventoryRecord(vv, c.userPubkeys)

		// Look up and set the user id and username.
		var ok bool
		v.UserID, ok = c.userPubkeys[v.PublicKey]
		if ok {
			v.Username = c.getUsernameByID(v.UserID)
		} else {
			log.Infof("%v", spew.Sdump(c.userPubkeys))
			log.Errorf("user not found for public key %v, for invoice %v",
				v.PublicKey, v.CensorshipRecord.Token)
		}

		len := len(allInvoices)
		if len == 0 {
			allInvoices = append(allInvoices, v)
			continue
		}

		// Insertion sort from oldest to newest.
		idx := sort.Search(len, func(i int) bool {
			return v.Timestamp < allInvoices[i].Timestamp
		})

		allInvoices = append(allInvoices[:idx],
			append([]v1.InvoiceRecord{v},
				allInvoices[idx:]...)...)
	}

	c.RUnlock()

	// pageStarted stores whether or not it's okay to start adding
	// invoices to the array. If the after or before parameter is
	// supplied, we must find the beginning (or end) of the page first.
	pageStarted := (pr.After == "" && pr.Before == "")
	beforeIdx := -1
	invoices := make([]v1.InvoiceRecord, 0)

	// Iterate the invoices.
	for i := 0; i < len(allInvoices); i++ {
		invoice := allInvoices[i]

		// Filter by user if it's provided.
		if pr.UserID != "" && pr.UserID != invoice.UserID {
			continue
		}

		// Filter by the month and year.
		if pr.Month != 0 && pr.Month != invoice.Month {
			continue
		}
		if pr.Year != 0 && pr.Year != invoice.Year {
			continue
		}

		// Filter by the status.
		if val, ok := pr.StatusMap[invoice.Status]; !ok || !val {
			continue
		}

		if pageStarted {
			invoices = append(invoices, invoice)
			if len(invoices) >= v1.ListPageSize {
				break
			}
		} else if pr.After != "" {
			// The beginning of the page has been found, so
			// the next public invoice is added.
			pageStarted = invoice.CensorshipRecord.Token == pr.After
		} else if pr.Before != "" {
			// The end of the page has been found, so we'll
			// have to iterate in the other direction to
			// add the invoices; save the current index.
			if invoice.CensorshipRecord.Token == pr.Before {
				beforeIdx = i
				break
			}
		}
	}

	// If beforeIdx is set, the caller is asking for vetted invoices whose
	// last result is before the provided invoice.
	if beforeIdx >= 0 {
		for j := beforeIdx - 1; j >= 0; j-- {
			invoice := allInvoices[j]

			// Filter by user if it's provided.
			if pr.UserID != "" && pr.UserID != invoice.UserID {
				continue
			}

			// Filter by the status.
			if val, ok := pr.StatusMap[invoice.Status]; !ok || !val {
				continue
			}

			// The iteration direction is newest -> oldest,
			// so invoices are prepended to the array so
			// the result will be oldest -> newest.
			invoices = append([]v1.InvoiceRecord{invoice},
				invoices...)
			if len(invoices) >= v1.ListPageSize {
				break
			}
		}
	}

	return invoices
}
