package main

import (
	"encoding/json"
	"net/http"
	"reflect"

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
	reqType interface{},
	perm permission,
	requiresInventory bool,
) {
	reflectReqType := reflect.TypeOf(reqType)

	wrapper := func(w http.ResponseWriter, r *http.Request) {
		req := reflect.New(reflectReqType).Interface()

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
	reqType interface{},
	perm permission,
	requiresInventory bool,
) {
	reflectReqType := reflect.TypeOf(reqType)

	wrapper := func(w http.ResponseWriter, r *http.Request) {
		req := reflect.New(reflectReqType).Interface()

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
	c.addGetRoute(v1.RoutePolicy, c.HandlePolicy, v1.Policy{},
		permissionPublic, false)
	c.addPostRoute(v1.RouteRegister, c.HandleRegister, v1.Register{},
		permissionPublic, false)
	c.addPostRoute(v1.RouteLogin, c.HandleLogin, v1.Login{},
		permissionPublic, false)
	c.addRoute(http.MethodPost, v1.RouteLogout, c.HandleLogout,
		permissionPublic, false)
	c.addPostRoute(v1.RouteResetPassword, c.HandleResetPassword,
		v1.ResetPassword{}, permissionPublic, false)

	// Routes that require being logged in.
	c.addPostRoute(v1.RouteNewIdentity, c.HandleNewIdentity,
		v1.NewIdentity{}, permissionLogin, false)
	c.addPostRoute(v1.RouteVerifyNewIdentity, c.HandleVerifyNewIdentity,
		v1.VerifyNewIdentity{}, permissionLogin, false)
	c.addPostRoute(v1.RouteChangePassword, c.HandleChangePassword,
		v1.ChangePassword{}, permissionLogin, false)
	c.addPostRoute(v1.RouteSubmitInvoice, c.HandleSubmitInvoice,
		v1.SubmitInvoice{}, permissionLogin, true)
	c.addPostRoute(v1.RouteEditInvoice, c.HandleEditInvoice,
		v1.EditInvoice{}, permissionLogin, true)
	c.addGetRoute(v1.RouteInvoiceDetails, c.HandleInvoiceDetails,
		v1.InvoiceDetails{}, permissionLogin, true)
	c.addGetRoute(v1.RouteUserInvoices, c.HandleMyInvoices,
		v1.MyInvoices{}, permissionLogin, true)
	c.addPostRoute(v1.RouteEditUser, c.HandleEditUser, v1.EditUser{},
		permissionLogin, false)
	c.addGetRoute(v1.RouteUserDetails, c.HandleUserDetails, v1.UserDetails{},
		permissionLogin, false)
	c.addPostRoute(v1.RouteEditUserExtendedPublicKey,
		c.HandleEditUserExtendedPublicKey, v1.EditUserExtendedPublicKey{},
		permissionLogin, false)

	// Routes that require being logged in as an admin user.
	c.addPostRoute(v1.RouteInviteNewUser, c.HandleInviteNewUser,
		v1.InviteNewUser{}, permissionAdmin, false)
	c.addPostRoute(v1.RouteManageUser, c.HandleManageUser, v1.ManageUser{},
		permissionAdmin, false)
	c.addGetRoute(v1.RouteInvoices, c.HandleInvoices,
		v1.Invoices{}, permissionAdmin, true)
	c.addPostRoute(v1.RouteSetInvoiceStatus, c.HandleSetInvoiceStatus,
		v1.SetInvoiceStatus{}, permissionAdmin, true)
	c.addPostRoute(v1.RouteReviewInvoices, c.HandleReviewInvoices,
		v1.ReviewInvoices{}, permissionAdmin, true)
	c.addPostRoute(v1.RoutePayInvoices, c.HandlePayInvoices,
		v1.PayInvoices{}, permissionAdmin, true)
	c.addGetRoute(v1.RouteUsers, c.HandleUsers, v1.Users{},
		permissionAdmin, false)
}
