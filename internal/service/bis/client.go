// BIS SDMX CSV client — shared HTTP fetch helper for BIS Statistics API.
// Uses CSV format (Accept: application/vnd.sdmx.data+csv) as per BIS docs.
// Rate limit: undocumented; we apply 1 req/s safety via caller-side sequencing.
package bis

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	bisAPIBase  = "https://stats.bis.org/api/v2/data/BIS"
	csvTimeout  = 25 * time.Second
)

var csvClient = &http.Client{Timeout: csvTimeout} //nolint:gochecknoglobals

// fetchCSV fetches BIS SDMX data for the given dataset + series key (or "all")
// in CSV format and returns the raw body bytes.
//
// dataset examples: "WS_CBPOL", "WS_CREDIT_GAP", "WS_GLI"
// seriesKey examples: "all", "Q.US", "Q.XM+Q.GB"
// lastN: number of observations to fetch per series (0 = all)
func fetchCSV(ctx context.Context, dataset, seriesKey string, lastN int) ([]byte, error) {
	url := fmt.Sprintf("%s,%s,1.0/%s", bisAPIBase, dataset, seriesKey)
	if lastN > 0 {
		url = fmt.Sprintf("%s?lastNObservations=%d&format=csv", url, lastN)
	} else {
		url += "?format=csv"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build CSV request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")
	req.Header.Set("Accept", "application/vnd.sdmx.data+csv")

	resp, err := csvClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("BIS CSV fetch %s/%s: %w", dataset, seriesKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BIS CSV %s/%s: HTTP %d", dataset, seriesKey, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read BIS CSV body: %w", err)
	}

	return body, nil
}
