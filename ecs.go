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
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	v1Ticket "github.com/OpenShopChannel/V1TicketGenerator"
	"github.com/wii-tools/wadlib"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	QueryOwnedTitles = `SELECT owned_titles.title_id
		FROM owned_titles
		WHERE owned_titles.account_id = $1`

	QueryOwnedServiceTitles = `SELECT service_titles.reference_id, owned_titles.date_purchased, service_titles.item_id
		FROM service_titles, owned_titles
		WHERE service_titles.item_id = owned_titles.item_id
		AND service_titles.title_id = $1
		AND owned_titles.account_id = $2`

	AssociateTicketStatement = `INSERT INTO owned_titles (account_id, title_id, version, item_id, date_purchased)
		VALUES ($1, $2, $3, $4, $5)`

	// SharedBalanceAmount describes the maximum signed 32-bit integer value.
	// It is not an actual tracked points value, but exists to permit reuse.
	SharedBalanceAmount = math.MaxInt32

	// WiinoMaApplicationID is the title ID for the Japanese channel Wii no Ma.
	WiinoMaApplicationID = "000100014843494A"
	// WiinoMaServiceTitleID is the service ID used by Wii no Ma's theatre.
	WiinoMaServiceTitleID = "000101006843494A"
)

// contentAesKey is the AES key that is used to encrypt title contents.
var contentAesKey = [16]byte{0x72, 0x95, 0xDB, 0xC0, 0x47, 0x3C, 0x90, 0x0B, 0xB5, 0x94, 0x19, 0x9C, 0xB5, 0xBC, 0xD3, 0xDC}

func getBalance() Balance {
	return Balance{
		Amount:   SharedBalanceAmount,
		Currency: "POINTS",
	}
}

func checkDeviceStatus(e *Envelope) {
	e.AddCustomType(getBalance())
	e.AddKVNode("ForceSyncTime", "0")
	e.AddKVNode("ExtTicketTime", e.Timestamp())
	e.AddKVNode("SyncTime", e.Timestamp())
}

func notifyETicketsSynced(e *Envelope) {
	// TODO: Implement handling of synchronization timing
}

func listETickets(e *Envelope) {
	accountId, err := e.AccountId()
	if err != nil {
		e.Error(2, "missing account ID", err)
		return
	}

	rows, err := pool.Query(ctx, QueryOwnedTitles, accountId)
	if err != nil {
		log.Printf("error executing statement: %v\n", err)
		e.Error(2, "database error", errors.New("failed to execute db operation"))
		return
	}

	// Add all available titles for this account.
	defer rows.Close()
	for rows.Next() {
		var titleId string
		err = rows.Scan(&titleId)
		if err != nil {
			log.Printf("error executing statement: %v\n", err)
			e.Error(2, "database error", errors.New("failed to execute db operation"))
			return
		}

		// Report the version of the package actually installed. Synthesized
		// channels carry a raised TMD version so consoles replace an older
		// Open Shop Channel release. Titles without a stored package cannot be
		// reissued, so they are left out rather than failing the whole list.
		manifest, manifestErr := channelpkg.Load("./channels", strings.ToLower(titleId))
		if manifestErr != nil {
			continue
		}
		version := int(manifest.Version)

		e.AddCustomType(Tickets{
			TitleId: titleId,
			Version: version,

			// We do not support migration, ticket IDs, or revocation.
			TicketId:     "0",
			RevokeDate:   0,
			MigrateCount: 0,
			MigrateLimit: 0,
		})
	}

	e.AddKVNode("ForceSyncTime", "0")
	e.AddKVNode("ExtTicketTime", e.Timestamp())
	e.AddKVNode("SyncTime", e.Timestamp())
}

func getETickets(e *Envelope) {
	e.AddKVNode("ForceSyncTime", "0")
	e.AddKVNode("ExtTicketTime", e.Timestamp())
	e.AddKVNode("SyncTime", e.Timestamp())
}

func purchaseTitle(e *Envelope) {
	accountId, err := e.AccountId()
	if err != nil {
		e.Error(2, "missing account ID", err)
		return
	}

	tempItemId, err := e.getKey("ItemId")
	if err != nil {
		e.Error(2, "missing item ID", err)
		return
	}

	// Our struct takes an integer rather than a string.
	itemId, err := strconv.Atoi(tempItemId)
	if err != nil {
		e.Error(2, "invalid item ID", err)
		return
	}

	// Determine the title ID we're going to purchase.
	titleId, err := e.getKey("TitleId")
	if err != nil {
		e.Error(2, "missing title ID", err)
		return
	}

	ticket := new(bytes.Buffer)
	var ticketStruct wadlib.Ticket
	err = binary.Read(bytes.NewReader(wadlib.TicketTemplate), binary.BigEndian, &ticketStruct)
	if err != nil {
		// Should never happen but report
		e.Error(2, "error reading ticket template", err)
		return
	}

	// We will now formulate the ticket for this title.
	intTitleId, err := strconv.ParseUint(titleId, 16, 64)
	if err != nil {
		e.Error(2, "invalid title id", err)
		return
	}

	ticketStruct.TitleID = intTitleId

	// Title key is encrypted with the common key and current title ID
	ticketStruct.UpdateTitleKey(contentAesKey)

	version := 0
	certificateChain := wadlib.CertChainTemplate
	if titleId == WiinoMaServiceTitleID {
		// Wii no Ma needs the ticket to be in the v1 ticket format.
		// Update the ticket to reflect that.
		ticketStruct.FileVersion = 1
		ticketStruct.AccessTitleMask = math.MaxUint32
		ticketStruct.LicenseType = 5

		err = binary.Write(ticket, binary.BigEndian, ticketStruct)
		if err != nil {
			e.Error(2, "failed to create ticket", err)
			return
		}

		refId, err := e.getKey("ReferenceId")
		if err != nil {
			e.Error(2, "missing reference ID", err)
			return
		}

		// Convert reference ID to bytes
		refIdBytes, err := hex.DecodeString(refId)
		if err != nil {
			log.Printf("unexpected error converting reference id to bytes: %v", err)
			e.Error(2, "error purchasing", nil)
			return
		}

		var referenceId [16]byte
		copy(referenceId[:], refIdBytes)

		subscriptions := []v1Ticket.V1SubscriptionRecord{
			{
				ExpirationTime: uint32(time.Now().AddDate(0, 1, 0).Unix()),
				ReferenceID:    referenceId,
			},
		}

		// Query the database for other purchased items of the same title id.
		rows, err := pool.Query(ctx, QueryOwnedServiceTitles, titleId, accountId)
		if err != nil {
			log.Printf("unexpected error purchasing: %v", err)
			e.Error(2, "error purchasing", nil)
			return
		}

		defer rows.Close()
		for rows.Next() {
			var currentRefIdString string
			var purchasedTime time.Time
			err = rows.Scan(&currentRefIdString, &purchasedTime, nil)
			if err != nil {
				log.Printf("unexpected error purchasing: %v", err)
				e.Error(2, "error purchasing", nil)
				return
			}

			refIdBytes, err = hex.DecodeString(currentRefIdString)
			if err != nil {
				log.Printf("unexpected error converting reference id to bytes: %v", err)
				e.Error(2, "error purchasing", nil)
				return
			}

			var currentReferenceId [16]byte
			copy(currentReferenceId[:], refIdBytes)
			subscriptions = append(subscriptions, v1Ticket.V1SubscriptionRecord{
				ExpirationTime: uint32(purchasedTime.AddDate(0, 0, 30).Unix()),
				ReferenceID:    currentReferenceId,
			})
		}

		newTicket, err := v1Ticket.CreateV1Ticket(ticket.Bytes(), subscriptions)
		if err != nil {
			log.Printf("unexpected error creating v1Ticket: %v", err)
			e.Error(2, "error creating ticket", nil)
			return
		}

		ticket = bytes.NewBuffer(newTicket)
	} else {
		// A catalog entry alone is not an installable package. Only issue the
		// matching imported ticket after a validated package is available.
		// Validation is local on purpose: the storefront already decided that
		// this title is for sale, and hash-derived title IDs never exist in the
		// live Open Shop Channel API.
		packageID := strings.ToLower(titleId)
		manifest, packageErr := channelpkg.Load("./channels", packageID)
		if packageErr != nil {
			e.Error(2, "native channel package is not available", nil)
			return
		}

		version = int(manifest.Version)
		packageTicket, readErr := os.ReadFile(filepath.Join("./channels", packageID, "ticket.bin"))
		if readErr != nil {
			e.Error(2, "channel ticket is unavailable", nil)
			return
		}
		certificateChain, readErr = os.ReadFile(filepath.Join("./channels", packageID, "certs.bin"))
		if readErr != nil || len(certificateChain) == 0 {
			e.Error(2, "channel certificates are unavailable", nil)
			return
		}
		ticket = bytes.NewBuffer(packageTicket)
	}

	// Associate the given title ID with the user.
	_, err = pool.Exec(ctx, AssociateTicketStatement, accountId, titleId, version, itemId, time.Now().UTC())
	if err != nil {
		log.Printf("unexpected error purchasing: %v", err)
		e.Error(2, "error purchasing", nil)
		return
	}

	// The returned ticket is expected to have two other certificates associated.
	ticketString := b64(append(ticket.Bytes(), certificateChain...))

	e.AddCustomType(getBalance())
	e.AddCustomType(Transactions{
		TransactionId: "00000000",
		Date:          e.Timestamp(),
		Type:          "PURCHGAME",
		TotalPaid:     0,
		Currency:      "POINTS",
		ItemId:        itemId,
		ItemPricing: Prices{
			ItemId:      itemId,
			Price:       Price{Amount: 0, Currency: "POINTS"},
			Limits:      LimitStruct(PR),
			LicenseKind: PERMANENT,
		},
	})
	e.AddKVNode("SyncTime", e.Timestamp())

	e.AddKVNode("ETickets", ticketString)
	// Two cert types must be present.
	e.AddKVNode("Certs", b64(certificateChain))
	e.AddKVNode("Certs", b64(certificateChain))
	e.AddKVNode("TitleId", titleId)
}

func listPurchaseHistory(e *Envelope) {
	accountId, err := e.AccountId()
	if err != nil {
		e.Error(2, "missing account ID", err)
		return
	}

	titleId, err := e.getKey("ApplicationId")
	if err != nil {
		e.Error(2, "missing application ID", err)
		return
	}

	var transactions []Transactions
	if titleId == WiinoMaApplicationID {
		// We will query the database differently for Wii no Ma.
		rows, err := pool.Query(ctx, QueryOwnedServiceTitles, WiinoMaServiceTitleID, accountId)
		if err != nil {
			log.Printf("unexpected error querying owned service titles: %v", err)
			e.Error(2, "error purchasing", nil)
			return
		}

		defer rows.Close()
		for rows.Next() {
			var refId string
			var purchasedTime time.Time
			var itemId int
			err = rows.Scan(&refId, &purchasedTime, &itemId)
			if err != nil {
				log.Printf("unexpected error purchasing: %v", err)
				e.Error(2, "error purchasing", nil)
				return
			}

			transaction := Transactions{
				TransactionId: "00000000",
				// (Sketch) I don't know why but Wii no Ma won't acknowledge the entry if it isn't past a day from
				// purchase.
				Date:      strconv.Itoa(int(purchasedTime.AddDate(0, 0, -1).UnixMilli())),
				Type:      "PURCHGAME",
				TotalPaid: 0,
				Currency:  "POINTS",
				ItemId:    itemId,
				ItemPricing: Prices{
					ItemId: itemId,
					Price: Price{
						Amount:   0,
						Currency: "POINTS",
					},
					Limits:      LimitStruct(PR),
					LicenseKind: SERVICE,
				},
				TitleId:     WiinoMaServiceTitleID,
				ItemCode:    itemId,
				ReferenceId: refId,
			}

			transactions = append(transactions, transaction)
		}
	} else {
		transactions = append(transactions, Transactions{
			TransactionId: "00000000",
			// Is timestamp in milliseconds, placeholder one is Wed Oct 19 2022 18:02:46
			Date:      "1666202566218",
			Type:      "PURCHGAME",
			TotalPaid: 0,
			Currency:  "POINTS",
			ItemId:    0,
			ItemPricing: Prices{
				ItemId: 0,
				Price: Price{
					Amount:   0,
					Currency: "POINTS",
				},
				Limits:      LimitStruct(PR),
				LicenseKind: PERMANENT,
			},
			TitleId: "000101006843494A",
		})
	}

	e.AddCustomType(transactions)
	e.AddKVNode("ListResultTotalSize", strconv.Itoa(len(transactions)))
}

// genServiceUrl keeps every Shop service on the known-good storefront host.
// The console learns these URLs from GetECConfig, so separate DNS records are
// unnecessary and older clients only need to establish one compatible TLS path.
func genServiceUrl(service string, path string) string {
	return "https://oss-auth." + baseUrl + "/" + service + "/services/" + path
}

func getECConfig(e *Envelope) {
	contentUrl := "https://oss-auth." + baseUrl + "/ccs/download"
	e.AddKVNode("ContentPrefixURL", contentUrl)
	e.AddKVNode("UncachedContentPrefixURL", contentUrl)
	e.AddKVNode("SystemContentPrefixURL", contentUrl)
	e.AddKVNode("SystemUncachedContentPrefixURL", contentUrl)

	e.AddKVNode("EcsURL", genServiceUrl("ecs", "ECommerceSOAP"))
	e.AddKVNode("IasURL", genServiceUrl("ias", "IdentityAuthenticationSOAP"))
	e.AddKVNode("CasURL", genServiceUrl("cas", "CatalogingSOAP"))
	e.AddKVNode("NusURL", genServiceUrl("nus", "NetUpdateSOAP"))
}
