//	Copyright (C) 2018-2020 CornierKhan1
//
//	WiiSOAP is SOAP Server Software, designed specifically to handle Wii Shop Channel SOAP.
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU Affero General Public License as published
//    by the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU Affero General Public License for more details.
//
//    You should have received a copy of the GNU Affero General Public License
//    along with this program.  If not, see http://www.gnu.org/licenses/.

package main

import (
	"bufio"
	"crypto/md5"
	"errors"
	"fmt"
	wiino "github.com/RiiConnect24/wiino/golang"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"log"
	"math/rand"
	"os"
	"slices"
	"strconv"
)

const (
	PrepareUserStatement = `INSERT INTO userbase
		(device_id, device_token, device_token_hashed, account_id, region, serial_number) 
	VALUES ($1, $2, $3, $4, $5, $6)`
	SyncUserStatement = `SELECT 
		account_id, device_token, serial_number
	FROM userbase WHERE 
		region = $1 AND
		device_id = $2`
	CheckUserStatement = `SELECT
		1
	FROM userbase WHERE
		device_id = $1 AND
		serial_number = $2 AND
		region = $3`
)

func checkRegistration(e *Envelope) {
	serialNo, err := e.getKey("SerialNumber")
	if err != nil {
		e.Error(5, "missing serial number", err)
		return
	}

	// We'll utilize our sync user statement.
	query := pool.QueryRow(ctx, CheckUserStatement, e.DeviceId(), serialNo, e.Region())
	err = query.Scan(nil)

	// Formulate our response
	e.AddKVNode("OriginalSerialNumber", serialNo)

	if err != nil {
		// We're either unregistered, or a database error occurred.
		if err == pgx.ErrNoRows {
			e.AddKVNode("DeviceStatus", DeviceStatusUnregistered)
		} else {
			log.Printf("error executing statement: %v\n", err)
			e.Error(5, "server-side error", err)
		}
	} else {
		// No errors! We're safe.
		e.AddKVNode("DeviceStatus", DeviceStatusRegistered)
	}
}

func getChallenge(e *Envelope) {
	// The official Wii Shop Channel requests a Challenge from the server, and promptly disregards it.
	// (Sometimes, it may not request a challenge at all.) No attempt is made to validate the response.
	// It then uses another hard-coded value in place of this returned value entirely in any situation.
	// For this reason, we consider it irrelevant.
	e.AddKVNode("Challenge", SharedChallenge)
}

func getRegistrationInfo(e *Envelope) {
	// GetRegistrationInfo is SyncRegistration with authentication and an additional key.
	syncRegistration(e)

	// This _must_ be POINTS.
	// It does not appear to be observed by any known client,
	// but is sent by Nintendo in official requests.
	e.AddKVNode("Currency", "POINTS")
}

func getWhitelistedSerialNumbers() []string {
	file, err := os.Open("whitelist.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Read each line
	var sns []string
	for scanner.Scan() {
		sns = append(sns, scanner.Text())

	}

	// Check for errors during scanning
	if err = scanner.Err(); err != nil {
		panic(err)
	}

	return sns
}

func syncRegistration(e *Envelope) {
	var accountId int64
	var deviceToken string
	var serialNumber string

	user := pool.QueryRow(ctx, SyncUserStatement, e.Region(), e.DeviceId())
	err := user.Scan(&accountId, &deviceToken, &serialNumber)
	if err != nil {
		e.Error(7, "An error occurred querying the database.", err)
	}

	if whitelistEnabled && !slices.Contains(getWhitelistedSerialNumbers(), serialNumber) {
		// Since HTTP server runs on a separate Goroutine, this won't shut off the server,
		// rather kill communication with the requesting console
		panic(err)
	}

	e.AddKVNode("AccountId", strconv.FormatInt(accountId, 10))
	e.AddKVNode("DeviceToken", deviceToken)
	e.AddKVNode("DeviceTokenExpired", "false")
	e.AddKVNode("Country", e.Country())
	e.AddKVNode("ExtAccountId", "")
	e.AddKVNode("DeviceStatus", "R")
}

func register(e *Envelope) {
	deviceCode, err := e.getKey("DeviceCode")
	if err != nil {
		e.Error(7, "missing device code", err)
		return
	}

	registerRegion, err := e.getKey("RegisterRegion")
	if err != nil {
		e.Error(7, "missing registration region", err)
		return
	}
	if registerRegion != e.Region() {
		e.Error(7, "mismatched region", errors.New("region does not match registration region"))
		return
	}

	serialNo, err := e.getKey("SerialNumber")
	if err != nil {
		e.Error(7, "missing serial number", err)
		return
	}

	if whitelistEnabled && !slices.Contains(getWhitelistedSerialNumbers(), serialNo) {
		// Since HTTP server runs on a separate Goroutine, this won't shut off the server,
		// rather kill communication with the requesting console
		panic(err)
	}

	// Validate given friend code.
	userId, err := strconv.ParseUint(deviceCode, 10, 64)
	if err != nil {
		e.Error(7, "invalid friend code", err)
		return
	}
	if wiino.NWC24CheckUserID(userId) != 0 {
		e.Error(7, "invalid friend code", err)
		return
	}

	// Generate a random 9-digit number, padding zeros as necessary.
	accountId := rand.Int63n(999999999)

	// Generate a device token, 21 characters...
	deviceToken := RandString(21)
	// ...and then its md5, because the Wii sends this for most requests.
	md5DeviceToken := fmt.Sprintf("%x", md5.Sum([]byte(deviceToken)))

	// Insert all of our obtained values to the database...
	_, err = pool.Exec(ctx, PrepareUserStatement, e.DeviceId(), deviceToken, md5DeviceToken, accountId, e.Region(), serialNo)
	if err != nil {
		// It's okay if this isn't a PostgreSQL error, as perhaps other issues have come in.
		if driverErr, ok := err.(*pgconn.PgError); ok {
			if driverErr.Code == "23505" {
				e.Error(7, "database error", errors.New("user already exists"))
				return
			}
		}
		log.Printf("error executing statement: %v\n", err)
		e.Error(7, "database error", errors.New("failed to execute db operation"))
		return
	}

	fmt.Println("The request is valid! Responding...")
	e.AddKVNode("AccountId", strconv.FormatInt(accountId, 10))
	e.AddKVNode("DeviceToken", deviceToken)
	e.AddKVNode("DeviceTokenExpired", "false")
	e.AddKVNode("Country", e.Country())
	// Optionally, one can send back DeviceCode and ExtAccountId to update on device.
	// We send these back as-is regardless.
	e.AddKVNode("ExtAccountId", "")
	e.AddKVNode("DeviceCode", deviceCode)
}

func unregister(e *Envelope) {
	// how abnormal... ;3
}
