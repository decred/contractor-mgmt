package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	www "github.com/decred/contractor-mgmt/cmswww/api/v1"
	pd "github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/util"
)

// rpc makes an http request to the method and route provided, serializing
// the provided object as the request body.
func (c *cmswww) rpc(method string, route string, v interface{}) ([]byte, error) {
	var (
		requestBody []byte
		err         error
	)
	if v != nil {
		requestBody, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}

	fullRoute := c.cfg.RPCHost + route

	if c.client == nil {
		c.client, err = util.NewClient(false, c.cfg.RPCCert)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, fullRoute, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.RPCUser, c.cfg.RPCPass)
	r, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		var pdErrorReply www.PDErrorReply
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&pdErrorReply); err != nil {
			return nil, err
		}

		return nil, www.PDError{
			HTTPCode:   r.StatusCode,
			ErrorReply: pdErrorReply,
		}
	}

	responseBody := util.ConvertBodyToByteArray(r.Body, false)
	return responseBody, nil
}

// remoteInventory fetches the entire inventory of invoices from politeiad.
func (c *cmswww) remoteInventory() (*pd.InventoryReply, error) {
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}
	inv := pd.Inventory{
		Challenge:     hex.EncodeToString(challenge),
		IncludeFiles:  false,
		VettedCount:   0,
		BranchesCount: 0,
	}

	responseBody, err := c.rpc(http.MethodPost, pd.InventoryRoute, inv)
	if err != nil {
		return nil, err
	}

	var ir pd.InventoryReply
	err = json.Unmarshal(responseBody, &ir)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal InventoryReply: %v",
			err)
	}

	err = util.VerifyChallenge(c.cfg.Identity, challenge, ir.Response)
	if err != nil {
		return nil, err
	}

	return &ir, nil
}

// LoadInventory fetches the entire inventory of invoices from politeiad and
// caches it, sorted by most recent timestamp.
func (c *cmswww) LoadInventory() error {
	c.Lock()
	defer c.Unlock()

	if c.inventoryLoaded {
		return nil
	}

	// Fetch remote inventory.
	inv, err := c.remoteInventory()
	if err != nil {
		return fmt.Errorf("LoadInventory: %v", err)
	}

	err = c.initializeInventory(inv)
	if err != nil {
		return fmt.Errorf("initializeInventory: %v", err)
	}

	log.Infof("Adding %v invoices to the database",
		len(inv.Vetted)+len(inv.Branches))

	c.inventoryLoaded = true
	return nil
}
