package main

import (
	"fmt"

	v1 "github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
	pd "github.com/decred/politeia/politeiad/api/v1"
)

var (
	errRecordNotFound = fmt.Errorf("record not found")
)

type inventoryRecord struct {
	record    pd.Record                 // actual record
	invoiceMD BackendInvoiceMetadata    // invoice metadata
	changes   []BackendInvoiceMDChange  // changes metadata
	payments  []BackendInvoiceMDPayment // payments metadata
}

// newInventoryRecord adds a Politeia record to the database.
//
// This function must be called WITH the mutex held.
func (c *cmswww) newInventoryRecord(record pd.Record) error {
	dbInvoice, err := c.convertRecordToDatabaseInvoice(record)
	if err != nil {
		return err
	}

	return c.db.CreateInvoice(dbInvoice)
}

// initializeInventory loads the database with the current inventory of Politeia records.
//
// This function must be called WITH the mutex held.
func (c *cmswww) initializeInventory(inv *pd.InventoryReply) error {
	for _, v := range append(inv.Vetted, inv.Branches...) {
		err := c.newInventoryRecord(v)
		if err != nil {
			return err
		}
	}

	return nil
}

// getInvoice returns a single invoice by its token
func (c *cmswww) getInvoice(token string) (*v1.InvoiceRecord, error) {
	dbInvoice, err := c.db.GetInvoiceByToken(token)
	if err != nil {
		return nil, err
	}

	return convertDatabaseInvoiceToInvoice(dbInvoice), nil
}

// getInvoices returns a list of invoices that adheres to the requirements
// specified in the provided request.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) getInvoices(pr database.InvoicesRequest) ([]v1.InvoiceRecord, int, error) {
	dbInvoices, numMatches, err := c.db.GetInvoices(pr)
	if err != nil {
		return nil, 0, err
	}

	return convertDatabaseInvoicesToInvoices(dbInvoices), numMatches, nil
}
