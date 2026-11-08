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
	"context"
	crypto "crypto/rand"
	"encoding/xml"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"time"
)

const (
	// SharedChallenge represents a static value to this nonsensical challenge response system.
	// The given challenge must be 11 characters or less. Contents do not matter.
	SharedChallenge = "NintyWhyPls"
)

var baseUrl string
var pool *pgxpool.Pool
var ctx = context.Background()
var isDebug = false
var ignoreAuth = false
var whitelistEnabled = false

// checkError makes error handling not as ugly and inefficient.
func checkError(err error) {
	if err != nil {
		log.Fatalf("WiiSOAP forgot how to drive and suddenly crashed! Reason: %v\n", err)
	}
}

func main() {
	// Seed our random number generator before anything else.
	seed, err := crypto.Int(crypto.Reader, big.NewInt(math.MaxInt64))
	checkError(err)
	rand.Seed(seed.Int64())

	adminMode := len(os.Args) > 1 && os.Args[1] == "admin"
	if adminMode && (len(os.Args) == 2 || os.Args[2] == "help" || os.Args[2] == "-h") {
		os.Exit(adminMain(nil))
	}

	// Initial Start.
	if !adminMode {
		fmt.Println("Shroombrew 0.1.0\n[i] Reading the Config...")
	}

	// Check the config.
	ioconfig, err := ioutil.ReadFile("./config.xml")
	checkError(err)
	readConfig := Config{}
	err = xml.Unmarshal(ioconfig, &readConfig)
	checkError(err)

	fmt.Println("[i] Initializing core...")
	isDebug = readConfig.Debug
	if isDebug {
		ignoreAuth = readConfig.NoAuth
	}

	whitelistEnabled = readConfig.Whitelist

	// Start SQL.
	dbString := fmt.Sprintf("postgres://%s:%s@%s/%s", readConfig.SQLUser, readConfig.SQLPass, readConfig.SQLAddress, readConfig.SQLDB)
	dbConf, err := pgxpool.ParseConfig(dbString)
	checkError(err)
	pool, err = pgxpool.ConnectConfig(ctx, dbConf)
	checkError(err)

	// Ensure this PostgreSQL connection is valid.
	defer pool.Close()
	checkError(err)

	baseUrl = readConfig.BaseURL

	if adminMode {
		keyPath := os.Getenv("SHROOMBREW_SECRET_FILE")
		if keyPath == "" {
			keyPath = "./secrets.key"
		}
		if _, statErr := os.Stat(keyPath); statErr != nil {
			log.Fatalf("admin: %s not found. Run from the install directory (cd /opt/shroombrew); a new key would orphan every stored card and address hash.", keyPath)
		}
		checkError(initShopWallets())
		checkError(initShopPurchases())
		checkError(initSecrets())
		checkError(initCommunity())
		code := adminMain(os.Args[2:])
		pool.Close()
		os.Exit(code)
	}

	// Start the HTTP server.
	fmt.Printf("Starting HTTP connection (%s)...\nNot using the usual port for HTTP?\nBe sure to use a proxy, otherwise the Wii can't connect!\n", readConfig.Address)

	r := NewRoute()
	ecs := r.HandleGroup("ecs")
	{
		ecs.Authenticated("CheckDeviceStatus", checkDeviceStatus)
		ecs.Authenticated("NotifyETicketsSynced", notifyETicketsSynced)
		ecs.Authenticated("ListETickets", listETickets)
		ecs.Authenticated("GetETickets", getETickets)
		ecs.Authenticated("PurchaseTitle", purchaseTitle)
		ecs.Unauthenticated("GetECConfig", getECConfig)
		ecs.Authenticated("ListPurchaseHistory", listPurchaseHistory)
	}

	ias := r.HandleGroup("ias")
	{
		ias.Unauthenticated("CheckRegistration", checkRegistration)
		ias.Unauthenticated("GetChallenge", getChallenge)
		// GetRegistrationInfo must be unauthenticated: the console calls it after
		// CheckRegistration returns R but before it has a DeviceToken (lost ec.cfg).
		// It queries by device_id+region like SyncRegistration and returns the token.
		ias.Unauthenticated("GetRegistrationInfo", getRegistrationInfo)
		ias.Unauthenticated("SyncRegistration", syncRegistration)
		ias.Unauthenticated("Register", register)
		ias.Authenticated("Unregister", unregister)
	}

	cas := r.HandleGroup("cas")
	{
		cas.Authenticated("ListItems", listItems)
	}
	store, err := newStorefront("./catalog.json")
	checkError(err)
	checkError(initShopWallets())
	checkError(initShopPurchases())
	checkError(initDownloadLog())
	checkError(initPlayRewards())
	checkError(initSecrets())
	checkError(initCommunity())
	go cardJanitor()
	address := readConfig.Address
	if override := os.Getenv("SHROOMBREW_ADDR"); override != "" {
		address = override
	}
	// Browsers receive the permissioned reference snapshot. Native Wii clients
	// use the existing compatibility frontend and Shroombrew services.
	contentHandler := shopErrorPages(nativeContentHandler("./channels", store.nativeECHandler(store.nativeV2Handler(referenceSiteHandler("./reference-site", store.featureHandler(store.classicSiteHandler(r.Handle())))))))
	server := &http.Server{Addr: address, Handler: contentHandler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())

	// From here on out, all special cool things should go into their respective handler function.
}
