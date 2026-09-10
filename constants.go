package main

import "errors"

// LimitKinds represents various limits applied to the current ticket.
type LimitKinds int

const (
	// PR is presumably "purchased".
	PR LimitKinds = 0
	TR            = 1
	DR            = 2
	SR            = 3
	LR            = 4
	AT            = 10000
)

// LicenceKinds represents the various rights a user has to a title.
type LicenceKinds string

const (
	PERMANENT LicenceKinds = "PERMANENT"
	DEMO      LicenceKinds = "DEMO"
	TRIAL     LicenceKinds = "TRIAL"
	RENTAL    LicenceKinds = "RENTAL"
	SUBSCRIPT LicenceKinds = "SUBSCRIPT"
	SERVICE   LicenceKinds = "SERVICE"
)

func GetLicenceKind(kind string) (*LicenceKinds, error) {
	names := map[string]LicenceKinds{
		"PERMANENT": PERMANENT,
		"DEMO":      DEMO,
		"TRIAL":     TRIAL,
		"RENTAL":    RENTAL,
		"SUBSCRIPT": SUBSCRIPT,
		"SERVICE":   SERVICE,
	}

	if value, exists := names[kind]; exists {
		return &value, nil
	} else {
		return nil, errors.New("invalid LicenceKind")
	}
}

// LimitStruct returns a Limits struct filled for the given kind.
func LimitStruct(kind LimitKinds) Limits {
	names := map[LimitKinds]string{
		PR: "PR",
		TR: "TR",
		DR: "DR",
		SR: "SR",
		LR: "LR",
		AT: "AT",
	}

	return Limits{
		Limits:    kind,
		LimitKind: names[kind],
	}
}

// DeviceStatus represents the various statuses a device may have.
//
// These values do not appear to be directly checked by the client within the
// Wii Shop Channel, and are a generic string. We could utilize any value we wish.
// However, titles utilizing DLCs appear to check the raw values.
// For this reason, we mirror values from Nintendo.
type DeviceStatus string

const (
	DeviceStatusRegistered   = "R"
	DeviceStatusUnregistered = "U"
)

// TokenType represents a way to distinguish between ST- (unhashed)
// and WT- (hashed) device tokens.
type TokenType int

const (
	TokenTypeUnhashed = iota
	TokenTypeHashed
	TokenTypeInvalid
)
