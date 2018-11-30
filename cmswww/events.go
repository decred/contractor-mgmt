package main

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

// EventT is the type of event.
type EventT int

// EventManager manages listeners (channels) for different event types.
type EventManager struct {
	Listeners map[EventT][]chan interface{}
}

const (
	EventTypeInvalid EventT = iota
	EventTypeInvoiceStatusChange
	EventTypeInvoicePaid
	EventTypeUserManage
)

type EventDataInvoiceStatusChange struct {
	Invoice   *database.Invoice
	AdminUser *database.User
}

type EventDataInvoicePaid struct {
	Invoice *database.Invoice
	TxID    string
}

type EventDataUserManage struct {
	AdminUser  *database.User
	User       *database.User
	ManageUser *v1.ManageUser
}

func (c *cmswww) getInvoiceContractor(dbInvoice *database.Invoice) (*database.User, error) {
	contractor, err := c.db.GetUserById(dbInvoice.UserID)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch contractor for invoice: %v", err)
	}

	return contractor, nil
}

func (c *cmswww) getInvoiceAndContractor(token string) (*database.Invoice, *database.User, error) {
	invoice, err := c.db.GetInvoiceByToken(token)
	if err != nil {
		return nil, nil, err
	}

	contractor, err := c.getInvoiceContractor(invoice)
	if err != nil {
		return nil, nil, err
	}

	return invoice, contractor, nil
}

// fireEvent is a convenience wrapper for EventManager._fireEvent which
// holds the lock.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) fireEvent(eventType EventT, data interface{}) {
	c.Lock()
	defer c.Unlock()

	c.eventManager._fireEvent(eventType, data)
}

func (c *cmswww) initEventManager() {
	c.Lock()
	defer c.Unlock()

	c.eventManager = &EventManager{}

	c._setupInvoiceStatusChangeLogging()
	c._setupUserManageLogging()

	if c.cfg.SMTP == nil {
		return
	}

	c._setupInvoiceStatusChangeEmailNotification()
	c._setupInvoicePaidEmailNotification()
}

func (c *cmswww) _setupInvoiceStatusChangeEmailNotification() {
	ch := make(chan interface{})
	go func() {
		for d := range ch {
			data, ok := d.(EventDataInvoiceStatusChange)
			if !ok {
				log.Errorf("invalid event data")
				continue
			}

			contractor, err := c.getInvoiceContractor(data.Invoice)
			if err != nil {
				log.Errorf("cannot fetch contractor for invoice: %v", err)
				continue
			}

			switch data.Invoice.Status {
			case v1.InvoiceStatusApproved:
				err := c.emailInvoiceApprovedNotification(contractor,
					data.Invoice)
				if err != nil {
					log.Errorf("email contractor for approved invoice %v: %v",
						data.Invoice.Token, err)
				}
			case v1.InvoiceStatusRejected:
				err := c.emailInvoiceRejectedNotification(contractor,
					data.Invoice)
				if err != nil {
					log.Errorf("email contractor for rejected invoice %v: %v",
						data.Invoice.Token, err)
				}
			default:
			}
		}
	}()
	c.eventManager._register(EventTypeInvoiceStatusChange, ch)
}

func (c *cmswww) _setupInvoicePaidEmailNotification() {
	ch := make(chan interface{})
	go func() {
		for d := range ch {
			data, ok := d.(EventDataInvoicePaid)
			if !ok {
				log.Errorf("invalid event data")
				continue
			}

			contractor, err := c.getInvoiceContractor(data.Invoice)
			if err != nil {
				log.Errorf("cannot fetch contractor for invoice: %v", err)
				continue
			}

			err = c.emailInvoicePaidNotification(contractor, data.Invoice,
				data.TxID)
			if err != nil {
				log.Errorf("email contractor for paid invoice %v: %v",
					data.Invoice.Token, err)
			}
		}
	}()
	c.eventManager._register(EventTypeInvoiceStatusChange, ch)
}

func (c *cmswww) _setupInvoiceStatusChangeLogging() {
	ch := make(chan interface{})
	go func() {
		for d := range ch {
			data, ok := d.(EventDataInvoiceStatusChange)
			if !ok {
				log.Errorf("invalid event data")
				continue
			}

			if data.AdminUser == nil {
				// Some status changes can occur based on events, such as
				// a payment being confirmed on the blockchain.
				continue
			}

			// Log the action in the admin log.
			err := c.logAdminInvoiceAction(data.AdminUser, data.Invoice.Token,
				fmt.Sprintf("set invoice status to %v,%v",
					v1.InvoiceStatus[data.Invoice.Status]),
				data.Invoice.StatusChangeReason)

			if err != nil {
				log.Errorf("could not log action to file: %v", err)
			}
		}
	}()
	c.eventManager._register(EventTypeInvoiceStatusChange, ch)
}

func (c *cmswww) _setupUserManageLogging() {
	ch := make(chan interface{})
	go func() {
		for d := range ch {
			data, ok := d.(EventDataUserManage)
			if !ok {
				log.Errorf("invalid event data")
				continue
			}

			// Log the action in the admin log.
			err := c.logAdminUserAction(data.AdminUser, data.User,
				v1.UserManageAction[data.ManageUser.Action],
				data.ManageUser.Reason)
			if err != nil {
				log.Errorf("could not log action to file: %v", err)
			}
		}
	}()
	c.eventManager._register(EventTypeUserManage, ch)
}

// _register adds a listener channel for the given event type.
//
// This function must be called WITH the mutex held.
func (e *EventManager) _register(eventType EventT, listenerToAdd chan interface{}) {
	if e.Listeners == nil {
		e.Listeners = make(map[EventT][]chan interface{})
	}

	if _, ok := e.Listeners[eventType]; ok {
		e.Listeners[eventType] = append(e.Listeners[eventType], listenerToAdd)
	} else {
		e.Listeners[eventType] = []chan interface{}{listenerToAdd}
	}
}

// _unregister removes the given listener channel for the given event type.
//
// This function must be called WITH the mutex held.
func (e *EventManager) _unregister(eventType EventT, listenerToRemove chan interface{}) {
	listeners, ok := e.Listeners[eventType]
	if !ok {
		return
	}

	for i, listener := range listeners {
		if listener == listenerToRemove {
			e.Listeners[eventType] = append(e.Listeners[eventType][:i],
				e.Listeners[eventType][i+1:]...)
			break
		}
	}
}

// _fireEvent iterates all listener channels for the given event type and
// passes the given data to it.
//
// This function must be called WITH the mutex held.
func (e *EventManager) _fireEvent(eventType EventT, data interface{}) {
	listeners, ok := e.Listeners[eventType]
	if !ok {
		return
	}

	for _, listener := range listeners {
		go func(listener chan interface{}) {
			listener <- data
		}(listener)
	}
}
