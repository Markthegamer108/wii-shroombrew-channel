package main

import "log"

const QueryTitlesTableByPriceCode = `SELECT item_id, price FROM service_titles WHERE price_code = $1`

func listItems(e *Envelope) {
	titleId, err := e.getKey("TitleId")
	if err != nil {
		e.Error(9, "Unable to obtain title.", err)
	}

	attrs, err := e.getKeys("AttributeFilters")
	if err != nil {
		e.Error(5, "AttributeFilters key did not exist!", err)
	}

	var licenceStr string
	var pricingCode string
	for _, attr := range attrs {
		name, value := parseNameValue(attr.InnerText())
		if name == "TitleKind" {
			licenceStr = value
		} else if name == "PricingCode" {
			pricingCode = value
		}
	}

	// Now validate
	licenceKind, err := GetLicenceKind(licenceStr)
	if err != nil {
		e.Error(5, "Invalid TitleKind was passed by SOAP", err)
	}

	// Query the titles table to get our title
	row := pool.QueryRow(ctx, QueryTitlesTableByPriceCode, pricingCode)

	var itemId int
	var price int
	err = row.Scan(&itemId, &price)
	if err != nil {
		log.Printf("error while querying titles table: %v", err)
		e.Error(2, "error retrieving title", nil)
		return
	}

	e.AddKVNode("ListResultTotalSize", "1")
	e.AddCustomType(Items{
		TitleId: titleId,
		Contents: ContentsMetadata{
			TitleIncluded: false,
			ContentIndex:  0,
		},
		Attributes: []Attributes{
			{
				Name:  "TitleVersion",
				Value: "0",
			},
			{
				Name:  "Prices",
				Value: "1",
			},
		},
		Ratings: Ratings{
			Name:   "E",
			Rating: 1,
			Age:    9,
		},
		Prices: Prices{
			ItemId: itemId,
			Price: Price{
				Amount:   price,
				Currency: "POINTS",
			},
			Limits:      LimitStruct(PR),
			LicenseKind: *licenceKind,
		},
	})
}
