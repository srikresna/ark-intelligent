package sec

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// informationTable represents the 13F XML holdings structure.
type informationTable struct {
	XMLName xml.Name       `xml:"informationTable"`
	Entries []infoTableRow `xml:"infoTable"`
}

type infoTableRow struct {
	Issuer     string `xml:"nameOfIssuer"`
	TitleClass string `xml:"titleOfClass"`
	CUSIP      string `xml:"cusip"`
	Value      string `xml:"value"`
	ShrOrPrn   struct {
		Amount  string `xml:"sshPrnamt"`
		AmtType string `xml:"sshPrnamtType"` // SH or PRN
	} `xml:"shrsOrPrnAmt"`
	PutCall string `xml:"putCall"`
}

// parseHoldingsXML parses a 13F information table XML into a slice of Holdings.
func parseHoldingsXML(data []byte) ([]Holding, error) {
	var table informationTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("13F XML parse: %w", err)
	}

	holdings := make([]Holding, 0, len(table.Entries))
	for _, e := range table.Entries {
		value, _ := strconv.ParseFloat(strings.ReplaceAll(e.Value, ",", ""), 64)
		shares, _ := strconv.ParseFloat(strings.ReplaceAll(e.ShrOrPrn.Amount, ",", ""), 64)

		holdings = append(holdings, Holding{
			Issuer:     e.Issuer,
			CUSIPClass: e.CUSIP,
			TitleClass: e.TitleClass,
			Value:      value,
			Shares:     shares,
			PutCall:    strings.ToUpper(e.PutCall),
		})
	}

	return holdings, nil
}

// BigMoveThresholdK is the value in $thousands above which a new position is
// considered a "significant move" that should trigger an alert.
// $1B = 1_000_000 thousands.
const BigMoveThresholdK float64 = 1_000_000

// SignificantNewPosition holds details about a large new 13F position.
type SignificantNewPosition struct {
	Institution string
	Issuer      string
	ValueK      float64 // in $thousands
}

// DetectSignificantMoves scans all institution reports and returns new positions
// whose value exceeds BigMoveThresholdK ($1B).
func DetectSignificantMoves(data *EdgarData) []SignificantNewPosition {
	var alerts []SignificantNewPosition
	for _, report := range data.Reports {
		for _, np := range report.NewPositions {
			if np.CurrValue >= BigMoveThresholdK {
				alerts = append(alerts, SignificantNewPosition{
					Institution: report.Institution.Name,
					Issuer:      np.Issuer,
					ValueK:      np.CurrValue,
				})
			}
		}
	}
	return alerts
}
