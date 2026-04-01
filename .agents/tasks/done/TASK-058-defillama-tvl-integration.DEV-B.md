# TASK-058: DeFiLlama Total TVL Integration

**Priority:** 🟡 MEDIUM  
**Cycle:** Siklus 2 — Data & Integrasi  
**Effort:** ~2 jam  
**Assignee:** Dev-B atau Dev-C

---

## Context

DeFiLlama menyediakan DeFi TVL data secara gratis, no API key:

**Endpoints verified live:**
```
GET https://api.llama.fi/v2/historicalChainTvl
→ Array [{date: unix, tvl: float}] — total semua DeFi
→ Saat ini: ~$94.4 billion (April 2026)

GET https://api.llama.fi/v2/chains
→ TVL per blockchain (Ethereum, BSC, Solana, dll)
```

DeFi TVL adalah proxy untuk:
- Risk appetite di crypto market
- Institutional participation di DeFi
- Market health (TVL naik = money flowing in)

Relevan untuk `/cryptoalpha` command dan sebagai konteks tambahan di `/sentiment`.

---

## Acceptance Criteria

### 1. DeFiLlama Client (`internal/service/marketdata/defillama/client.go`)

```go
package defillama

type TVLPoint struct {
    Date  time.Time
    TVL   float64 // in USD
}

type TVLSummary struct {
    Current    float64   // latest TVL
    Change7D   float64   // % change last 7 days
    Change30D  float64   // % change last 30 days
    Trend      string    // "EXPANDING" | "CONTRACTING" | "STABLE"
    FetchedAt  time.Time
    Available  bool
}

// FetchHistoricalTVL fetches total DeFi TVL history (all chains)
func FetchHistoricalTVL(ctx context.Context) (*TVLSummary, error)
```

Parse from `https://api.llama.fi/v2/historicalChainTvl`:
- Take last 31 entries (30 day history)
- Compute current TVL, 7d change %, 30d change %
- Classify trend: EXPANDING (>5% 7d), CONTRACTING (<-5% 7d), STABLE

### 2. Integrate ke Crypto Context

Tambah ke `internal/service/marketdata/coingecko/client.go` atau buat wrapper di service layer:
- Fetch TVL summary saat `/cryptoalpha` command
- Cache 6 jam (TVL update harian)

### 3. Display di `/cryptoalpha` Output

Tambah ke formatter output:
```
🏗️ DeFi TVL: $94.4B  (+2.3% 7d) — STABLE
```

### 4. Inject ke AI Context

Di `internal/service/ai/unified_outlook.go`, inject jika available:
```go
if tvl.Available {
    b.WriteString(fmt.Sprintf("DeFi TVL: $%.1fB (%+.1f%% 7d) — %s\n", 
        tvl.Current/1e9, tvl.Change7D, tvl.Trend))
}
```

---

## API Spec

```
GET https://api.llama.fi/v2/historicalChainTvl
No auth. No rate limit documented (be respectful, cache aggressively).

Response: JSON array
[
  {"date": 1774915200, "tvl": 94402257599},
  ...
]
```

---

## Files to Create/Edit

- `internal/service/marketdata/defillama/client.go` — NEW
- `internal/adapter/telegram/formatter.go` — tambah TVL ke FormatCryptoAlpha
- `internal/service/ai/unified_outlook.go` — inject TVL context

---

## Notes

- No API key needed
- Rate limit: tidak ada documented limit — gunakan cache 6 jam minimum
- TVL dalam USD raw (integer) — bagi 1e9 untuk Billions display
- Tidak perlu chain-level breakdown untuk MVP — total saja
- Error handling: jika API down, skip gracefully (Available = false)
