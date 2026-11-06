package main

import (
	"github.com/jackc/pgx/v4"
	"github.com/logrusorgru/aurora/v3"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// Route defines a header to be checked for actions, and an array of actions to handle.
type Route struct {
	HeaderName string
	Actions    []Action
}

// Action contains information about how a specified action should be handled.
type Action struct {
	ActionName          string
	Callback            func(e *Envelope)
	NeedsAuthentication bool
	ServiceType         string
}

// NewRoute produces a new route struct with appropriate header defaults.
func NewRoute() Route {
	return Route{
		HeaderName: "SOAPAction",
	}
}

// RoutingGroup defines a group of actions for a given service type.
type RoutingGroup struct {
	Route       *Route
	ServiceType string
}

// HandleGroup returns a routing group type for the given service type.
func (r *Route) HandleGroup(serviceType string) RoutingGroup {
	return RoutingGroup{
		Route:       r,
		ServiceType: serviceType,
	}
}

// Unauthenticated associates an action to a function to be handled without authentication.
func (r *RoutingGroup) Unauthenticated(action string, function func(e *Envelope)) {
	r.Route.Actions = append(r.Route.Actions, Action{
		ActionName:          action,
		Callback:            function,
		NeedsAuthentication: false,
		ServiceType:         r.ServiceType,
	})
}

// Authenticated associates an action to a function to be handled with authentication.
func (r *RoutingGroup) Authenticated(action string, function func(e *Envelope)) {
	r.Route.Actions = append(r.Route.Actions, Action{
		ActionName:          action,
		Callback:            function,
		NeedsAuthentication: true,
		ServiceType:         r.ServiceType,
	})
}

func (route *Route) Handle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log route metadata only; SOAP bodies contain device authentication tokens.
		log.Printf("%s %s via %s", aurora.Yellow(r.Method), aurora.Cyan(r.URL.Path), aurora.Cyan(r.Host))
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

		if r.Method != "POST" {
			printError(w, "WiiSOAP can't handle this. Try again later.")
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			printError(w, "Error reading request body...")
			return
		}

		// Check if there's a header of the type we need.
		service, actionName := parseAction(r.Header.Get("SOAPAction"))
		if service == "" || actionName == "" {
			service, actionName = sniffActionFromXML(string(body))
		}

		if service == "" || actionName == "" {
			printError(w, "WiiSOAP can't handle this. Try again later.")
			return
		}

		// Verify this is a service type we know.
		switch service {
		case "ecs":
		case "ias":
		case "cas":
			break
		default:
			printError(w, "Unsupported service type...")
			return
		}

		debugPrint("[!] Incoming ", aurora.Yellow(strings.ToUpper(service)), " request - handling request ", aurora.Yellow(actionName))

		// Ensure we can route to this action before processing.
		// Search all registered actions and find a matching action.
		var action Action
		for _, routeAction := range route.Actions {
			if routeAction.ActionName == actionName && routeAction.ServiceType == service {
				action = routeAction
			}
		}

		// Action is only properly populated if we found it previously.
		if action.ActionName == "" && action.ServiceType == "" {
			printError(w, "WiiSOAP can't handle this. Try again later.")
			return
		}

		// Insert the current action being performed.
		e, err := NewEnvelope(service, actionName, body)
		if err != nil {
			printError(w, "Error interpreting request body: "+err.Error())
			return
		}

		// Check for authentication.
		if action.NeedsAuthentication {
			success, authErr := checkAuthentication(e)
			// Catch-all in case of invalid formatting or true invalidity.
			if !success || authErr != nil {
				// Report the problem inside a normal SOAP reply. A bare HTTP
				// 401 makes the console show an HTTP-status code (204701)
				// instead of the shop's own error screen.
				e.Error(901, "authentication failed", authErr)
			} else {
				action.Callback(e)
			}
		} else {
			// Call this action.
			action.Callback(e)
		}

		// Nintendo's services answered with HTTP 200 even when the SOAP body
		// carried an ErrorCode. The console's EC library only maps those codes
		// to the familiar 205xxx screens if it can parse the body, so keep the
		// transport successful and let ErrorCode carry the failure.
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, contents := e.becomeXML()
		w.Write([]byte(contents))
	})
}

const (
	RouteVerifyHashedStatement   = `SELECT 1 FROM userbase WHERE device_token_hashed=$1 AND account_id=$2 AND device_id=$3`
	RouteVerifyUnhashedStatement = `SELECT 1 FROM userbase WHERE device_token=$1 AND account_id=$2 AND device_id=$3`
)

// checkAuthentication validates various factors from a given request requiring authentication.
func checkAuthentication(e *Envelope) (bool, error) {
	if ignoreAuth {
		return true, nil
	}

	// Get necessary authentication identifiers.
	deviceToken, err := e.getKey("DeviceToken")
	if err != nil {
		return false, err
	}
	accountId, err := e.AccountId()
	if err != nil {
		return false, err
	}

	hash, tokenType := determineTokenFormat(deviceToken)
	if hash == "" || tokenType == TokenTypeInvalid {
		return false, nil
	}

	var statement string
	if tokenType == TokenTypeHashed {
		statement = RouteVerifyHashedStatement
	} else if tokenType == TokenTypeUnhashed {
		statement = RouteVerifyUnhashedStatement
	}

	// Check using various input given.
	row := pool.QueryRow(ctx, statement, hash, accountId, e.DeviceId())

	var throwaway int
	err = row.Scan(&throwaway)
	if err == pgx.ErrNoRows {
		return false, err
	} else if err != nil {
		// We shouldn't encounter other errors.
		debugPrint("error occurred while checking authentication: ", err)
		return false, err
	} else {
		return true, nil
	}
}

// validateTokenFormat confirms the prefix, size and type of tokens,
// which are expected to be in a format such as
// WT-5d41402abc4b2a76b9719d911017c592 or ST-aech1kae4sheequ8Zohwa.
// It returns an empty string on failure, alongside TokenTypeInvalid.
func determineTokenFormat(token string) (string, TokenType) {
	tokenLen := len(token)
	if tokenLen < 3 {
		return "", TokenTypeInvalid
	}

	switch token[:3] {
	case "ST-":
		// Unhashed tokens are 24 characters in length.
		if tokenLen == 24 {
			return token[3:24], TokenTypeUnhashed
		}
	case "WT-":
		// Hashed tokens are 35 characters in length.
		if tokenLen == 35 {
			return token[3:35], TokenTypeHashed
		}
	}

	return "", TokenTypeInvalid
}

func printError(w http.ResponseWriter, reason string) {
	http.Error(w, reason, http.StatusInternalServerError)
	debugPrint("Failed to handle request: ", aurora.Red(reason))
}
