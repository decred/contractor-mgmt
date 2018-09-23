package main

import (
	"encoding/json"
	"net/http"

	"github.com/decred/politeia/util"
	"github.com/gorilla/mux"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

type routeHandlerFunc func(
	interface{},
	*database.User,
	http.ResponseWriter,
	*http.Request,
) (interface{}, error)

// addRoute sets up a handler for a specific method+route.
func (c *cmswww) addRoute(
	method string,
	route string,
	handler http.HandlerFunc,
	perm permission,
	requiresInventory bool,
) {
	fullRoute := v1.APIRoute + route

	if requiresInventory {
		handler = c.loadInventory(handler)
	}
	switch perm {
	case permissionAdmin:
		handler = logging(c.isLoggedInAsAdmin(handler))
	case permissionLogin:
		handler = logging(c.isLoggedIn(handler))
	default:
		handler = logging(handler)
	}

	// All handlers need to close the body
	handler = closeBody(handler)

	c.router.StrictSlash(true).HandleFunc(fullRoute, handler).Methods(method)
}

// addGetRoute sets up a handler for a GET route.
func (c *cmswww) addGetRoute(
	route string,
	handler routeHandlerFunc,
	req interface{},
	perm permission,
	requiresInventory bool,
) {
	wrapper := func(w http.ResponseWriter, r *http.Request) {
		// Get the command.
		err := util.ParseGetParams(r, req)
		if err != nil {
			RespondWithError(w, r, 0, "",
				v1.UserError{
					ErrorCode: v1.ErrorStatusInvalidInput,
				})
			return
		}

		user, err := c.GetSessionUser(r)
		if err != nil && err != database.ErrUserNotFound {
			RespondWithError(w, r, 0,
				"route handler: GetSessionUser %v", err)
			return
		}

		resp, err := handler(req, user, w, r)
		if err != nil {
			RespondWithError(w, r, 0, "route handler: %v", err)
			return
		}

		util.RespondWithJSON(w, http.StatusOK, resp)
	}

	c.addRoute(http.MethodGet, route, wrapper, perm, requiresInventory)
}

// addPostRoute sets up a handler for a POST route.
func (c *cmswww) addPostRoute(
	route string,
	handler routeHandlerFunc,
	req interface{},
	perm permission,
	requiresInventory bool,
) {
	wrapper := func(w http.ResponseWriter, r *http.Request) {
		// Get the command.
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(req); err != nil {
			RespondWithError(w, r, 0, "route handler: failed to decode: %v", err)
			return
		}

		user, err := c.GetSessionUser(r)
		if err != nil && err != database.ErrUserNotFound {
			RespondWithError(w, r, 0,
				"route handler: GetSessionUser %v", err)
			return
		}

		resp, err := handler(req, user, w, r)
		if err != nil {
			RespondWithError(w, r, 0, "route handler: %v", err)
			return
		}

		util.RespondWithJSON(w, http.StatusOK, resp)
	}

	c.addRoute(http.MethodPost, route, wrapper, perm, requiresInventory)
}

func (c *cmswww) SetupRoutes() {
	c.router = mux.NewRouter()

	// Public routes.
	c.router.HandleFunc("/", closeBody(logging(c.HandleVersion))).Methods(http.MethodGet)
	c.router.NotFoundHandler = closeBody(c.HandleNotFound)
	c.addGetRoute(v1.RoutePolicy, c.HandlePolicy, new(v1.Policy),
		permissionPublic, false)
	c.addPostRoute(v1.RouteRegister, c.HandleRegister, new(v1.Register),
		permissionPublic, false)
	c.addPostRoute(v1.RouteLogin, c.HandleLogin, new(v1.Login),
		permissionPublic, false)
	c.addRoute(http.MethodPost, v1.RouteLogout, c.HandleLogout,
		permissionPublic, false)
	c.addPostRoute(v1.RouteResetPassword, c.HandleResetPassword,
		new(v1.ResetPassword), permissionPublic, false)

	// Routes that require being logged in.
	c.addPostRoute(v1.RouteNewIdentity, c.HandleNewIdentity,
		new(v1.NewIdentity), permissionLogin, false)
	c.addPostRoute(v1.RouteVerifyNewIdentity, c.HandleVerifyNewIdentity,
		new(v1.VerifyNewIdentity), permissionLogin, false)
	c.addPostRoute(v1.RouteChangePassword, c.HandleChangePassword,
		new(v1.ChangePassword), permissionLogin, false)
	c.addPostRoute(v1.RouteSubmitInvoice, c.HandleSubmitInvoice,
		new(v1.SubmitInvoice), permissionLogin, true)
	c.addGetRoute(v1.RouteInvoiceDetails, c.HandleInvoiceDetails,
		new(v1.InvoiceDetails), permissionLogin, true)
	c.addGetRoute(v1.RouteUserInvoices, c.HandleMyInvoices,
		new(v1.MyInvoices), permissionLogin, true)

	// Routes that require being logged in as an admin user.
	c.addPostRoute(v1.RouteInviteNewUser, c.HandleInviteNewUser,
		new(v1.InviteNewUser), permissionAdmin, false)
	c.addGetRoute(v1.RouteUserDetails, c.HandleUserDetails, new(v1.UserDetails),
		permissionAdmin, false)
	c.addPostRoute(v1.RouteEditUser, c.HandleEditUser, new(v1.EditUser),
		permissionAdmin, false)
	c.addGetRoute(v1.RouteInvoices, c.HandleInvoices,
		new(v1.Invoices), permissionAdmin, true)
	c.addPostRoute(v1.RouteSetInvoiceStatus, c.HandleSetInvoiceStatus,
		new(v1.SetInvoiceStatus), permissionAdmin, true)
}
