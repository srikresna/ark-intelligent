#!/usr/bin/env python3
"""
Quant Engine — Econometric/Statistical Analysis for Trading.

Usage: python3 quant_engine.py <input.json> <output.json> [chart_output.png]

Modes: stats, garch, correlation, regime, seasonal, granger, meanrevert, pca, cointegration, var, risk, full

Input JSON: { mode, symbol, timeframe, bars[], multi_asset{}, params{} }
Output JSON: { mode, symbol, success, error, result{}, chart_path, text_output }
"""

import json
import sys
import warnings
import traceback
from datetime import datetime

import numpy as np
import pandas as pd

warnings.filterwarnings("ignore")

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker

# ---------------------------------------------------------------------------
# Theme
# ---------------------------------------------------------------------------
BG_COLOR = "#0e1117"
TEXT_COLOR = "#c9d1d9"
GRID_COLOR = "#1e2530"
UP_COLOR = "#26a69a"
DOWN_COLOR = "#ef5350"
ACCENT1 = "#42A5F5"
ACCENT2 = "#FFD700"
ACCENT3 = "#AB47BC"
ACCENT4 = "#FF8C00"

plt.rcParams.update({
    "figure.facecolor": BG_COLOR,
    "axes.facecolor": BG_COLOR,
    "axes.edgecolor": GRID_COLOR,
    "axes.labelcolor": TEXT_COLOR,
    "xtick.color": TEXT_COLOR,
    "ytick.color": TEXT_COLOR,
    "text.color": TEXT_COLOR,
    "grid.color": GRID_COLOR,
    "grid.linestyle": ":",
    "font.size": 9,
})


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def bars_to_df(bars: list) -> pd.DataFrame:
    """Convert bar list to DataFrame with DatetimeIndex."""
    rows = []
    for b in bars:
        o, h, l, c = float(b["open"]), float(b["high"]), float(b["low"]), float(b["close"])
        if o <= 0 or h <= 0 or l <= 0 or c <= 0:
            continue
        rows.append({
            "Date": pd.Timestamp(b["date"]),
            "Open": o, "High": h, "Low": l, "Close": c,
            "Volume": float(b.get("volume", 0)),
        })
    if not rows:
        return pd.DataFrame()
    df = pd.DataFrame(rows).set_index("Date").sort_index()
    return df


def compute_returns(df: pd.DataFrame, col: str = "Close") -> pd.Series:
    """Log returns."""
    return np.log(df[col] / df[col].shift(1)).dropna()


def annualization_factor(timeframe: str) -> float:
    """Return annualization multiplier for Sharpe etc."""
    tf = timeframe.lower()
    mapping = {"15m": 252*24*4, "30m": 252*24*2, "1h": 252*24,
               "4h": 252*6, "6h": 252*4, "12h": 252*2, "daily": 252, "d": 252}
    return mapping.get(tf, 252)


def save_chart(fig, path):
    fig.savefig(path, dpi=100, bbox_inches="tight", facecolor=BG_COLOR)
    plt.close(fig)


def safe_float(v):
    """Convert to JSON-safe float (handle NaN/Inf)."""
    if v is None or (isinstance(v, float) and (np.isnan(v) or np.isinf(v))):
        return None
    return round(float(v), 6)


def output(mode, symbol, success, result, text_output, error="", chart_path=""):
    return {
        "mode": mode,
        "symbol": symbol,
        "success": success,
        "error": error,
        "result": result,
        "text_output": text_output,
        "chart_path": chart_path,
    }


# ===========================================================================
# MODE: STATS — Distribution Analysis
# ===========================================================================

def compute_stats(df, symbol, timeframe, params, chart_path=None):
    from scipy import stats as sp_stats

    returns = compute_returns(df)
    n = len(returns)
    if n < 30:
        return output("stats", symbol, False, {}, "", error="Minimal 30 bar untuk analisis statistik")

    ann = annualization_factor(timeframe)
    mean_r = returns.mean()
    std_r = returns.std()
    skew = float(sp_stats.skew(returns))
    kurt = float(sp_stats.kurtosis(returns, fisher=True))  # excess kurtosis

    # Jarque-Bera test
    jb_stat, jb_p = sp_stats.jarque_bera(returns)
    is_normal = jb_p > 0.05

    # Annualized
    ann_return = mean_r * ann
    ann_vol = std_r * np.sqrt(ann)
    sharpe = ann_return / ann_vol if ann_vol > 0 else 0
    downside = returns[returns < 0].std() * np.sqrt(ann)
    sortino = ann_return / downside if downside > 0 else 0

    # VaR
    var_95 = float(np.percentile(returns, 5))
    var_99 = float(np.percentile(returns, 1))
    cvar_95 = float(returns[returns <= var_95].mean()) if len(returns[returns <= var_95]) > 0 else var_95

    # Recent stats (30d)
    recent = returns[-30:] if len(returns) >= 30 else returns
    recent_vol = recent.std() * np.sqrt(ann)

    result = {
        "n_observations": n,
        "mean_return": safe_float(mean_r),
        "std_dev": safe_float(std_r),
        "skewness": safe_float(skew),
        "kurtosis": safe_float(kurt),
        "jarque_bera_stat": safe_float(jb_stat),
        "jarque_bera_p": safe_float(jb_p),
        "is_normal": is_normal,
        "ann_return": safe_float(ann_return),
        "ann_volatility": safe_float(ann_vol),
        "recent_vol_30d": safe_float(recent_vol),
        "sharpe": safe_float(sharpe),
        "sortino": safe_float(sortino),
        "var_95": safe_float(var_95),
        "var_99": safe_float(var_99),
        "cvar_95": safe_float(cvar_95),
        "max_return": safe_float(returns.max()),
        "min_return": safe_float(returns.min()),
    }

    # Chart: histogram + QQ plot
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 5))
        fig.suptitle(f"{symbol} — Statistical Profile — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Histogram
        ax1.hist(returns, bins=50, color=ACCENT1, alpha=0.7, edgecolor="none", density=True)
        # Overlay normal distribution
        x = np.linspace(returns.min(), returns.max(), 100)
        ax1.plot(x, sp_stats.norm.pdf(x, mean_r, std_r), color=DOWN_COLOR, linewidth=1.5, label="Normal dist")
        ax1.axvline(var_95, color=ACCENT2, linestyle="--", linewidth=1, label=f"VaR 95%: {var_95:.4f}")
        ax1.axvline(0, color=GRID_COLOR, linewidth=0.5)
        ax1.set_title("Return Distribution", color=TEXT_COLOR)
        ax1.set_xlabel("Log Return")
        ax1.set_ylabel("Density")
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # QQ Plot
        sorted_returns = np.sort(returns)
        theoretical_q = sp_stats.norm.ppf(np.linspace(0.01, 0.99, len(sorted_returns)))
        ax2.scatter(theoretical_q, sorted_returns, s=3, color=ACCENT1, alpha=0.6)
        lims = [min(theoretical_q.min(), sorted_returns.min()), max(theoretical_q.max(), sorted_returns.max())]
        ax2.plot(lims, lims, color=DOWN_COLOR, linewidth=1, linestyle="--", label="Normal line")
        ax2.set_title("Q-Q Plot (vs Normal)", color=TEXT_COLOR)
        ax2.set_xlabel("Theoretical Quantiles")
        ax2.set_ylabel("Sample Quantiles")
        ax2.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    # Format text
    normal_str = "✅ Normal" if is_normal else "❌ Non-Normal (fat tails)"
    skew_str = "left tail risk ⚠️" if skew < -0.3 else ("right tail" if skew > 0.3 else "simetris")

    text = f"""📊 <b>Statistical Profile: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━

📈 <b>Returns ({n} observasi, {timeframe}):</b>
  Mean: {mean_r*100:+.4f}%  |  Std Dev: {std_r*100:.4f}%
  Skewness: {skew:.2f} ({skew_str})
  Kurtosis: {kurt:.2f} {"(fat tails ⚠️)" if kurt > 3 else "(normal tails)"}
  Jarque-Bera: p={jb_p:.4f} → {normal_str}

💰 <b>Annualized:</b>
  Return: {ann_return*100:+.2f}%  |  Vol: {ann_vol*100:.2f}%
  Sharpe: {sharpe:.2f}  |  Sortino: {sortino:.2f}
  Recent Vol (30d): {recent_vol*100:.2f}%

🎯 <b>Value at Risk:</b>
  VaR (95%): {var_95*100:.3f}%/bar
  VaR (99%): {var_99*100:.3f}%/bar
  CVaR (95%): {cvar_95*100:.3f}%/bar
  → $10,000 portfolio: max daily loss ~${abs(var_95)*10000:.0f}

📉 <b>Extremes:</b>
  Best: {returns.max()*100:+.3f}%  |  Worst: {returns.min()*100:.3f}%"""

    return output("stats", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: GARCH — Volatility Analysis & Forecast
# ===========================================================================

def compute_garch(df, symbol, timeframe, params, chart_path=None):
    from arch import arch_model

    returns = compute_returns(df) * 100  # arch uses percentage returns
    n = len(returns)
    if n < 50:
        return output("garch", symbol, False, {}, "", error="Minimal 50 bar untuk GARCH")

    ann = annualization_factor(timeframe)

    # Fit GARCH(1,1)
    model = arch_model(returns, vol="Garch", p=1, q=1, dist="t", rescale=False)
    res = model.fit(disp="off", show_warning=False)

    # Current conditional volatility
    cond_vol = res.conditional_volatility
    current_vol = cond_vol.iloc[-1] / 100  # back to decimal
    ann_current_vol = current_vol * np.sqrt(ann)

    # Forecast
    horizon = params.get("forecast_horizon", 5)
    fcast = res.forecast(horizon=horizon)
    forecast_var = fcast.variance.iloc[-1].values  # array of h-step variances
    forecast_vol = np.sqrt(forecast_var) / 100  # to decimal

    # Persistence
    alpha = float(res.params.get("alpha[1]", 0))
    beta = float(res.params.get("beta[1]", 0))
    persistence = alpha + beta

    # Realized vol at different windows
    raw_returns = compute_returns(df)
    rv_10 = raw_returns[-10:].std() * np.sqrt(ann) if n >= 10 else 0
    rv_20 = raw_returns[-20:].std() * np.sqrt(ann) if n >= 20 else 0
    rv_60 = raw_returns[-60:].std() * np.sqrt(ann) if n >= 60 else 0

    # Vol percentile (1yr)
    if n >= 252:
        rolling_vol = raw_returns.rolling(20).std() * np.sqrt(ann)
        rolling_vol = rolling_vol.dropna()
        vol_pct = float(sp_percentileofscore(rolling_vol.values, rv_20))
    else:
        rolling_vol = raw_returns.rolling(20).std() * np.sqrt(ann)
        rolling_vol = rolling_vol.dropna()
        vol_pct = float(sp_percentileofscore(rolling_vol.values, rv_20)) if len(rolling_vol) > 10 else 50.0

    # Vol regime
    if vol_pct > 75:
        vol_regime = "🔴 HIGH"
    elif vol_pct > 40:
        vol_regime = "🟡 MEDIUM"
    else:
        vol_regime = "🟢 LOW"

    result = {
        "current_daily_vol": safe_float(current_vol),
        "ann_current_vol": safe_float(ann_current_vol),
        "forecast_vol": [safe_float(v) for v in forecast_vol],
        "persistence": safe_float(persistence),
        "alpha": safe_float(alpha),
        "beta": safe_float(beta),
        "rv_10d": safe_float(rv_10),
        "rv_20d": safe_float(rv_20),
        "rv_60d": safe_float(rv_60),
        "vol_percentile": safe_float(vol_pct),
        "vol_regime": vol_regime,
    }

    # Chart
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(14, 8), gridspec_kw={"height_ratios": [2, 1]})
        fig.suptitle(f"{symbol} — Volatility Analysis (GARCH) — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Price + vol overlay
        dates = df.index[-len(cond_vol):]
        ax1.plot(dates, df["Close"].iloc[-len(cond_vol):], color=ACCENT1, linewidth=1, label="Price")
        ax1.set_ylabel("Price", color=TEXT_COLOR)
        ax1.legend(loc="upper left", fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # Conditional volatility
        ax2.fill_between(dates, 0, cond_vol.values, color=ACCENT3, alpha=0.4)
        ax2.plot(dates, cond_vol.values, color=ACCENT3, linewidth=1, label="GARCH Cond. Vol (%)")
        # Forecast
        last_date = dates[-1]
        fc_dates = pd.date_range(start=last_date, periods=horizon+1, freq="B")[1:]
        ax2.plot(fc_dates[:len(forecast_vol)], forecast_vol * 100 * np.sqrt(ann),
                color=ACCENT2, linewidth=2, linestyle="--", label="Forecast")
        ax2.set_ylabel("Volatility (%)", color=TEXT_COLOR)
        ax2.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""📈 <b>Volatility Analysis: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━

🎯 <b>Current Regime: {vol_regime}</b>
  Vol Percentile (1yr): {vol_pct:.0f}th

📊 <b>Realized Volatility (annualized):</b>
  RV (10d): {rv_10*100:.2f}%
  RV (20d): {rv_20*100:.2f}%
  RV (60d): {rv_60*100:.2f}%

🌊 <b>GARCH(1,1) Model:</b>
  Current cond. vol: {current_vol*100:.4f}%/bar
  Annualized: {ann_current_vol*100:.2f}%
  Persistence (α+β): {persistence:.4f}
  {"⚠️ Sangat persisten — vol clustering kuat" if persistence > 0.95 else "✅ Normal persistence"}

📉 <b>Forecast ({horizon} bar ke depan):</b>"""

    for i, v in enumerate(forecast_vol[:horizon]):
        text += f"\n  Bar +{i+1}: {v*100:.4f}%"

    text += f"""

💡 <b>Trading Implication:</b>
  {"📍 Vol tinggi → perbesar SL, kurangi size" if vol_pct > 75 else "📍 Vol normal → standard position sizing" if vol_pct > 40 else "📍 Vol rendah → breakout potential, perketat SL"}"""

    return output("garch", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: CORRELATION — Cross-Asset Correlation Matrix
# ===========================================================================

def compute_correlation(df, symbol, timeframe, params, multi_asset, chart_path=None):
    lookback = params.get("lookback", 120)

    # Build multi-asset return matrix
    all_closes = {}
    # Primary symbol
    returns_primary = compute_returns(df)
    all_closes[symbol] = returns_primary

    for sym, bars in (multi_asset or {}).items():
        if not bars:
            continue
        closes = pd.Series(
            [float(b["close"]) for b in bars],
            index=[pd.Timestamp(b["date"]) for b in bars],
            name=sym,
        ).sort_index()
        ret = np.log(closes / closes.shift(1)).dropna()
        if len(ret) > 20:
            all_closes[sym] = ret

    if len(all_closes) < 2:
        return output("correlation", symbol, False, {}, "", error="Minimal 2 aset untuk correlation matrix")

    # Align dates
    ret_df = pd.DataFrame(all_closes)
    ret_df = ret_df.dropna()
    ret_df = ret_df.iloc[-lookback:] if len(ret_df) > lookback else ret_df

    if len(ret_df) < 20:
        return output("correlation", symbol, False, {}, "", error="Insufficient overlapping data")

    corr = ret_df.corr()
    symbols = list(corr.columns)

    # Rolling correlation (primary vs each)
    rolling_corrs = {}
    for sym in symbols:
        if sym == symbol:
            continue
        rc = ret_df[symbol].rolling(30).corr(ret_df[sym]).dropna()
        if len(rc) > 0:
            rolling_corrs[sym] = {
                "current": safe_float(rc.iloc[-1]),
                "prev_30d": safe_float(rc.iloc[-31]) if len(rc) > 31 else None,
                "mean": safe_float(rc.mean()),
            }

    # Build result
    corr_dict = {}
    for s1 in symbols:
        corr_dict[s1] = {}
        for s2 in symbols:
            corr_dict[s1][s2] = safe_float(corr.loc[s1, s2])

    result = {
        "symbols": symbols,
        "correlation_matrix": corr_dict,
        "rolling_correlations": rolling_corrs,
        "n_observations": len(ret_df),
    }

    # Chart: heatmap
    if chart_path:
        import seaborn as sns
        fig, ax = plt.subplots(figsize=(10, 8))
        fig.suptitle(f"Correlation Matrix — {timeframe} ({len(ret_df)}d)", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        mask = np.triu(np.ones_like(corr, dtype=bool), k=1)
        cmap = sns.diverging_palette(10, 150, as_cmap=True)
        sns.heatmap(corr, mask=mask, cmap=cmap, vmin=-1, vmax=1, center=0,
                   annot=True, fmt=".2f", linewidths=0.5, linecolor=GRID_COLOR,
                   ax=ax, square=True, cbar_kws={"shrink": 0.8},
                   annot_kws={"size": 8, "color": TEXT_COLOR})
        ax.tick_params(colors=TEXT_COLOR)
        plt.tight_layout()
        save_chart(fig, chart_path)

    # Format text
    text = f"""🔗 <b>Correlation: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 {len(ret_df)} observasi, {timeframe}
(Lihat chart untuk heatmap lengkap)

🔑 <b>Key Correlations:</b>
"""
    # Sort by absolute correlation with primary symbol, show top relationships
    sorted_corrs = sorted(
        [(sym, rc) for sym, rc in rolling_corrs.items() if rc.get("current") is not None],
        key=lambda x: abs(x[1]["current"]), reverse=True
    )
    for sym, rc in sorted_corrs[:8]:
        curr = rc["current"]
        strength = "sangat kuat" if abs(curr) > 0.7 else ("moderate" if abs(curr) > 0.4 else "lemah")
        direction = "positif" if curr > 0 else "negatif"
        emoji = "🟢" if abs(curr) > 0.5 else "🟡"
        text += f"  {emoji} {symbol}↔{sym}: {curr:+.2f} ({strength} {direction})\n"
        if rc.get("prev_30d") is not None:
            delta = curr - rc["prev_30d"]
            trend = "strengthening" if abs(curr) > abs(rc["prev_30d"]) else "weakening"
            text += f"    30d change: {delta:+.2f} ({trend})\n"

    return output("correlation", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: REGIME — Hidden Markov Model
# ===========================================================================

def compute_regime(df, symbol, timeframe, params, chart_path=None):
    # Try hmmlearn first; fall back to sklearn GaussianMixture (similar regime detection)
    try:
        from hmmlearn.hmm import GaussianHMM
        USE_HMM = True
    except ImportError:
        from sklearn.mixture import GaussianMixture
        USE_HMM = False

    returns = compute_returns(df)
    n = len(returns)
    if n < 60:
        return output("regime", symbol, False, {}, "", error="Minimal 60 bar untuk regime detection")

    n_regimes = params.get("n_regimes", 3)
    ann = annualization_factor(timeframe)

    # Features: returns + absolute returns (vol proxy)
    features = np.column_stack([returns.values, np.abs(returns.values)])

    # Fit model (HMM or GaussianMixture fallback)
    best_model = None
    best_score = -np.inf
    for _ in range(10):  # multiple restarts for stability
        try:
            if USE_HMM:
                model = GaussianHMM(n_components=n_regimes, covariance_type="full",
                                  n_iter=200, random_state=np.random.randint(10000),
                                  tol=1e-4)
                model.fit(features)
                score = model.score(features)
            else:
                model = GaussianMixture(n_components=n_regimes, covariance_type="full",
                                       n_init=1, random_state=np.random.randint(10000))
                model.fit(features)
                score = -model.bic(features)  # lower BIC = better, negate for max
            if score > best_score:
                best_score = score
                best_model = model
        except Exception:
            continue

    if best_model is None:
        return output("regime", symbol, False, {}, "", error="HMM fitting gagal — data mungkin tidak cukup")

    states = best_model.predict(features)
    state_probs = best_model.predict_proba(features)

    # Characterize each regime
    regime_info = []
    for i in range(n_regimes):
        mask = states == i
        if mask.sum() == 0:
            regime_info.append({"id": i, "mean": 0, "vol": 0, "count": 0})
            continue
        r = returns.values[mask]
        regime_info.append({
            "id": i,
            "mean": float(np.mean(r)),
            "vol": float(np.std(r)) * np.sqrt(ann),
            "count": int(mask.sum()),
            "pct": float(mask.sum() / n * 100),
        })

    # Sort by vol: low vol → high vol
    regime_info.sort(key=lambda x: x["vol"])

    # Assign labels
    labels = ["🟢 Low Vol (Bull)", "🟡 Transition", "🔴 High Vol (Bear)"]
    if n_regimes == 2:
        labels = ["🟢 Low Vol", "🔴 High Vol"]

    regime_map = {}  # old_id → new_idx
    for new_idx, ri in enumerate(regime_info):
        regime_map[ri["id"]] = new_idx
        ri["label"] = labels[new_idx] if new_idx < len(labels) else f"Regime {new_idx}"

    # Current regime
    current_state = states[-1]
    current_idx = regime_map[current_state]
    current_prob = float(state_probs[-1, current_state])
    current_label = regime_info[current_idx]["label"]

    # Duration in current regime
    duration = 1
    for j in range(len(states)-2, -1, -1):
        if states[j] == current_state:
            duration += 1
        else:
            break

    # Transition matrix (HMM only; GaussianMixture uses uniform proxy)
    reorder = [ri["id"] for ri in regime_info]
    if USE_HMM and hasattr(best_model, 'transmat_'):
        trans_mat = best_model.transmat_
        trans_reordered = trans_mat[np.ix_(reorder, reorder)]
    else:
        # Estimate transitions empirically from state sequence
        trans_reordered = np.full((n_regimes, n_regimes), 1.0 / n_regimes)
        for idx in range(len(states) - 1):
            i_old = regime_map[states[idx]]
            i_new = regime_map[states[idx + 1]]
            trans_reordered[i_old, i_new] += 1
        row_sums = trans_reordered.sum(axis=1, keepdims=True)
        row_sums[row_sums == 0] = 1
        trans_reordered = trans_reordered / row_sums

    result = {
        "current_regime": current_idx,
        "current_label": current_label,
        "current_probability": safe_float(current_prob),
        "duration_bars": duration,
        "regimes": [{
            "idx": i,
            "label": ri["label"],
            "mean_return": safe_float(ri["mean"]),
            "ann_vol": safe_float(ri["vol"]),
            "pct_time": safe_float(ri.get("pct", 0)),
        } for i, ri in enumerate(regime_info)],
        "transition_matrix": [[safe_float(v) for v in row] for row in trans_reordered.tolist()],
    }

    # Chart: price colored by regime
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(14, 8), gridspec_kw={"height_ratios": [3, 1]})
        fig.suptitle(f"{symbol} — Market Regime (HMM) — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        colors = [UP_COLOR, ACCENT2, DOWN_COLOR]
        dates = df.index[1:]  # match returns length
        prices = df["Close"].iloc[1:]

        # Background coloring by regime
        mapped_states = [regime_map[s] for s in states]
        for i in range(len(dates)-1):
            ax1.axvspan(dates[i], dates[i+1], alpha=0.15,
                       color=colors[mapped_states[i] % len(colors)])
        ax1.plot(dates, prices, color=TEXT_COLOR, linewidth=1)
        ax1.set_ylabel("Price")
        ax1.grid(True, alpha=0.2)

        # Legend
        for i, ri in enumerate(regime_info):
            ax1.plot([], [], color=colors[i % len(colors)], linewidth=8, alpha=0.4,
                    label=f"{ri['label']} ({ri.get('pct', 0):.0f}%)")
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR, loc="upper left")

        # Regime probability
        for i, ri in enumerate(regime_info):
            prob_series = state_probs[:, ri["id"]]
            ax2.fill_between(dates, 0, prob_series, alpha=0.3, color=colors[i % len(colors)])
        ax2.set_ylabel("Regime Probability")
        ax2.set_ylim(0, 1)
        ax2.grid(True, alpha=0.2)

        plt.tight_layout()
        save_chart(fig, chart_path)

    # Format text
    text = f"""🎭 <b>Market Regime: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━

🎯 <b>Current: {current_label}</b>
  Probability: {current_prob*100:.0f}%
  Duration: {duration} bar

📊 <b>Regime Properties:</b>
"""
    for ri in regime_info:
        text += f"  {ri['label']}: μ={ri['mean']*100:+.3f}%/bar, σ={ri['vol']*100:.1f}% ann, {ri.get('pct', 0):.0f}% of time\n"

    text += f"""
🔄 <b>Transition Probabilities (from current):</b>
"""
    for i, ri in enumerate(regime_info):
        prob = trans_reordered[current_idx][i]
        text += f"  → {ri['label']}: {prob*100:.0f}%\n"

    # Trading implication
    if current_idx == 0:
        impl = "✅ LONG bias — low vol bull regime. Full position size."
    elif current_idx == len(regime_info) - 1:
        impl = "⚠️ CAUTION — high vol bear regime. Reduce size atau cash."
    else:
        impl = "🔄 TRANSITION — mixed signals. Standard size, tight stops."

    text += f"\n💡 <b>Trading Implication:</b>\n  {impl}"

    return output("regime", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: MEAN REVERSION TEST — ADF + Hurst
# ===========================================================================

def compute_meanrevert(df, symbol, timeframe, params, chart_path=None):
    from statsmodels.tsa.stattools import adfuller

    close = df["Close"]
    returns = compute_returns(df)
    n = len(close)
    if n < 50:
        return output("meanrevert", symbol, False, {}, "", error="Minimal 50 bar untuk mean reversion test")

    # ADF test
    adf_result = adfuller(close.values, maxlag=int(np.sqrt(n)), regression="c")
    adf_stat, adf_p = adf_result[0], adf_result[1]

    # ADF on returns (should be stationary)
    adf_ret = adfuller(returns.values, maxlag=int(np.sqrt(len(returns))))
    adf_ret_stat, adf_ret_p = adf_ret[0], adf_ret[1]

    # Hurst exponent (R/S method)
    def hurst_rs(ts, max_k=None):
        ts = np.array(ts)
        n = len(ts)
        if max_k is None:
            max_k = min(n // 4, 100)
        ks = range(10, max_k)
        rs_values = []
        for k in ks:
            rs_k = []
            for start in range(0, n - k, k):
                sub = ts[start:start+k]
                mean_sub = np.mean(sub)
                cumdev = np.cumsum(sub - mean_sub)
                r = np.max(cumdev) - np.min(cumdev)
                s = np.std(sub, ddof=1)
                if s > 0:
                    rs_k.append(r / s)
            if rs_k:
                rs_values.append((np.log(k), np.log(np.mean(rs_k))))
        if len(rs_values) < 2:
            return 0.5
        x, y = zip(*rs_values)
        slope, _ = np.polyfit(x, y, 1)
        return float(slope)

    hurst = hurst_rs(returns.values)

    # Half-life of mean reversion (for prices if mean-reverting)
    half_life = None
    if adf_p < 0.05:
        from statsmodels.regression.linear_model import OLS
        lag = close.shift(1).dropna()
        delta = close.diff().dropna()
        idx = lag.index.intersection(delta.index)
        if len(idx) > 10:
            try:
                result_ols = OLS(delta.loc[idx], lag.loc[idx]).fit()
                if result_ols.params.iloc[0] < 0:
                    half_life = -np.log(2) / result_ols.params.iloc[0]
            except Exception:
                pass

    # Z-score
    lookback_z = min(params.get("lookback", 50), n)
    mean_price = close.rolling(lookback_z).mean()
    std_price = close.rolling(lookback_z).std()
    zscore = ((close - mean_price) / std_price).dropna()

    # Interpretation
    if hurst < 0.4:
        hurst_interp = "🔄 Mean-Reverting — strategi reversion cocok"
    elif hurst > 0.6:
        hurst_interp = "📈 Trending — strategi momentum/breakout cocok"
    else:
        hurst_interp = "🔀 Random Walk — tidak ada edge clear"

    if adf_p < 0.01:
        adf_interp = "✅ Stationary (p≤0.01) — mean reversion kuat"
    elif adf_p < 0.05:
        adf_interp = "✅ Stationary (p≤0.05) — mean reversion moderate"
    elif adf_p < 0.10:
        adf_interp = "⚠️ Marginal (p≤0.10) — weak evidence"
    else:
        adf_interp = "❌ Non-Stationary — trending/random walk"

    result = {
        "adf_statistic": safe_float(adf_stat),
        "adf_p_value": safe_float(adf_p),
        "adf_return_p": safe_float(adf_ret_p),
        "hurst_exponent": safe_float(hurst),
        "half_life": safe_float(half_life) if half_life else None,
        "current_zscore": safe_float(zscore.iloc[-1]) if len(zscore) > 0 else None,
        "is_mean_reverting": adf_p < 0.05 and hurst < 0.5,
        "is_trending": hurst > 0.6,
    }

    # Chart
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(14, 7), gridspec_kw={"height_ratios": [2, 1]})
        fig.suptitle(f"{symbol} — Mean Reversion Test — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Price + rolling mean
        ax1.plot(close.index, close, color=ACCENT1, linewidth=1, label="Price")
        ax1.plot(mean_price.index, mean_price, color=ACCENT4, linewidth=1, linestyle="--", label=f"SMA({lookback_z})")
        ax1.set_ylabel("Price")
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # Z-score
        ax2.fill_between(zscore.index, 0, zscore, where=zscore > 0, alpha=0.3, color=UP_COLOR)
        ax2.fill_between(zscore.index, 0, zscore, where=zscore < 0, alpha=0.3, color=DOWN_COLOR)
        ax2.plot(zscore.index, zscore, color=TEXT_COLOR, linewidth=0.8)
        ax2.axhline(2, color=DOWN_COLOR, linestyle="--", linewidth=0.7, alpha=0.7)
        ax2.axhline(-2, color=UP_COLOR, linestyle="--", linewidth=0.7, alpha=0.7)
        ax2.axhline(0, color=GRID_COLOR, linewidth=0.5)
        ax2.set_ylabel("Z-Score")
        ax2.set_ylim(-3.5, 3.5)
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    z_current = zscore.iloc[-1] if len(zscore) > 0 else 0

    if z_current > 2:
        z_interp = "🔴 Overbought — reversal probability tinggi"
    elif z_current < -2:
        z_interp = "🟢 Oversold — bounce probability tinggi"
    else:
        z_interp = "🟡 Normal range"

    ret_stationary = "✅ stationary" if adf_ret_p < 0.05 else "❌ non-stationary"

    text = f"""🔄 <b>Mean Reversion Test: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 <b>ADF Test (Augmented Dickey-Fuller):</b>
  Price: stat={adf_stat:.3f}, p={adf_p:.4f}
  → {adf_interp}
  Returns: p={adf_ret_p:.4f} {ret_stationary}

📈 <b>Hurst Exponent: {hurst:.3f}</b>
  → {hurst_interp}
  (0.5=random walk, &lt;0.5=mean-revert, &gt;0.5=trending)

📉 <b>Z-Score (current): {z_current:+.2f}</b>
  {z_interp}"""

    if half_life is not None:
        text += f"\n\n⏱️ <b>Half-life: {half_life:.1f} bar</b>\n  → Mean revert dalam ~{half_life:.0f} bar"

    return output("meanrevert", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: GRANGER — Granger Causality Test
# ===========================================================================

def compute_granger(df, symbol, timeframe, params, multi_asset, chart_path=None):
    from statsmodels.tsa.stattools import grangercausalitytests

    max_lag = params.get("max_lag", 5)
    returns_primary = compute_returns(df)

    results_all = {}
    for sym, bars in (multi_asset or {}).items():
        if not bars or len(bars) < 50:
            continue
        closes = pd.Series(
            [float(b["close"]) for b in bars],
            index=[pd.Timestamp(b["date"]) for b in bars],
        ).sort_index()
        ret = np.log(closes / closes.shift(1)).dropna()

        # Align
        aligned = pd.DataFrame({symbol: returns_primary, sym: ret}).dropna()
        if len(aligned) < 50:
            continue

        # Test: does sym Granger-cause symbol?
        try:
            test_data = aligned[[symbol, sym]].values
            gc = grangercausalitytests(test_data, maxlag=max_lag, verbose=False)
            best_lag = min(gc, key=lambda k: gc[k][0]["ssr_ftest"][1])
            best_p = gc[best_lag][0]["ssr_ftest"][1]
            results_all[sym] = {
                "best_lag": best_lag,
                "best_p_value": safe_float(best_p),
                "causes": best_p < 0.05,
                "direction": f"{sym} → {symbol}",
            }
        except Exception:
            continue

        # Also test reverse: does symbol Granger-cause sym?
        try:
            test_data_rev = aligned[[sym, symbol]].values
            gc_rev = grangercausalitytests(test_data_rev, maxlag=max_lag, verbose=False)
            best_lag_rev = min(gc_rev, key=lambda k: gc_rev[k][0]["ssr_ftest"][1])
            best_p_rev = gc_rev[best_lag_rev][0]["ssr_ftest"][1]
            results_all[f"{sym}_rev"] = {
                "best_lag": best_lag_rev,
                "best_p_value": safe_float(best_p_rev),
                "causes": best_p_rev < 0.05,
                "direction": f"{symbol} → {sym}",
            }
        except Exception:
            continue

    if not results_all:
        return output("granger", symbol, False, {}, "", error="Tidak cukup data multi-aset untuk Granger test")

    result = {"tests": results_all}

    # Chart: bar chart of p-values
    if chart_path:
        fig, ax = plt.subplots(figsize=(12, 6))
        fig.suptitle(f"Granger Causality — {symbol} — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        names = []
        p_vals = []
        colors = []
        for key, val in results_all.items():
            names.append(val["direction"])
            p_vals.append(val["best_p_value"] or 1.0)
            colors.append(UP_COLOR if val["causes"] else DOWN_COLOR)

        y_pos = range(len(names))
        ax.barh(y_pos, p_vals, color=colors, alpha=0.7)
        ax.axvline(0.05, color=ACCENT2, linestyle="--", linewidth=1.5, label="p=0.05 threshold")
        ax.set_yticks(y_pos)
        ax.set_yticklabels(names, fontsize=8)
        ax.set_xlabel("p-value")
        ax.legend(fontsize=8, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax.grid(True, alpha=0.3, axis="x")
        ax.invert_xaxis()

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""⚡ <b>Granger Causality: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
"Apakah X memprediksi {symbol}? Atau sebaliknya?"

"""
    sig_found = False
    for key, val in results_all.items():
        emoji = "✅" if val["causes"] else "❌"
        text += f"  {emoji} {val['direction']} (lag {val['best_lag']}, p={val['best_p_value']:.4f})\n"
        if val["causes"]:
            sig_found = True

    if not sig_found:
        text += "\n⚠️ Tidak ada hubungan Granger-causal yang signifikan."
    else:
        text += "\n💡 Aset dengan ✅ bisa jadi leading indicator!"

    return output("granger", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: SEASONAL — Day-of-Week & Month-of-Year Analysis
# ===========================================================================

def compute_seasonal(df, symbol, timeframe, params, chart_path=None):
    """Analyze historical returns by day-of-week and month-of-year."""

    returns = compute_returns(df)
    n = len(returns)
    if n < 60:
        return output("seasonal", symbol, False, {}, "", error="Minimal 60 bar untuk seasonal analysis")

    from scipy import stats as sp_stats

    # --- Day of Week Analysis ---
    dow_names = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
    returns_with_dow = returns.copy()
    returns_with_dow.index = pd.to_datetime(returns_with_dow.index)
    dow_groups = returns_with_dow.groupby(returns_with_dow.index.dayofweek)

    dow_stats = {}
    for dow, group in dow_groups:
        if len(group) < 5:
            continue
        mean_r = float(group.mean())
        std_r = float(group.std())
        win_rate = float((group > 0).mean())
        count = len(group)
        # T-test: is mean significantly different from 0?
        t_stat, p_val = sp_stats.ttest_1samp(group, 0)
        dow_stats[dow_names[dow]] = {
            "mean_return": safe_float(mean_r),
            "std_dev": safe_float(std_r),
            "win_rate": safe_float(win_rate),
            "count": count,
            "t_stat": safe_float(t_stat),
            "p_value": safe_float(p_val),
            "significant": p_val < 0.05,
        }

    # --- Month of Year Analysis ---
    month_names = ["Jan", "Feb", "Mar", "Apr", "May", "Jun",
                   "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
    month_groups = returns_with_dow.groupby(returns_with_dow.index.month)

    month_stats = {}
    for month, group in month_groups:
        if len(group) < 5:
            continue
        mean_r = float(group.mean())
        std_r = float(group.std())
        win_rate = float((group > 0).mean())
        count = len(group)
        t_stat, p_val = sp_stats.ttest_1samp(group, 0)
        month_stats[month_names[month - 1]] = {
            "mean_return": safe_float(mean_r),
            "std_dev": safe_float(std_r),
            "win_rate": safe_float(win_rate),
            "count": count,
            "t_stat": safe_float(t_stat),
            "p_value": safe_float(p_val),
            "significant": p_val < 0.05,
        }

    # --- Current context ---
    today = df.index[-1]
    current_dow = dow_names[today.dayofweek] if hasattr(today, 'dayofweek') else "N/A"
    current_month = month_names[today.month - 1] if hasattr(today, 'month') else "N/A"

    result = {
        "day_of_week": dow_stats,
        "month_of_year": month_stats,
        "current_day": current_dow,
        "current_month": current_month,
        "n_observations": n,
    }

    # Chart: two subplots — DOW bar chart + Month bar chart
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6))
        fig.suptitle(f"{symbol} — Seasonal Analysis — {timeframe} ({n} obs)",
                    color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Day of Week
        if dow_stats:
            days = [d for d in dow_names if d in dow_stats]
            means = [dow_stats[d]["mean_return"] * 100 for d in days]
            colors_dow = [UP_COLOR if m > 0 else DOWN_COLOR for m in means]
            bars1 = ax1.bar(days, means, color=colors_dow, alpha=0.7, edgecolor="none")
            ax1.axhline(0, color=GRID_COLOR, linewidth=0.5)
            ax1.set_title("Day of Week", color=TEXT_COLOR)
            ax1.set_ylabel("Mean Return (%)")
            ax1.grid(True, alpha=0.2, axis="y")
            # Mark significant days
            for i, d in enumerate(days):
                if dow_stats[d].get("significant"):
                    ax1.text(i, means[i], "★", ha="center", va="bottom" if means[i] > 0 else "top",
                            color=ACCENT2, fontsize=12, fontweight="bold")
            # Win rate labels
            for i, d in enumerate(days):
                wr = dow_stats[d]["win_rate"] * 100
                ax1.text(i, 0, f"{wr:.0f}%", ha="center", va="top" if means[i] > 0 else "bottom",
                        color=TEXT_COLOR, fontsize=7, alpha=0.7)

        # Month of Year
        if month_stats:
            months = [m for m in month_names if m in month_stats]
            means_m = [month_stats[m]["mean_return"] * 100 for m in months]
            colors_m = [UP_COLOR if m > 0 else DOWN_COLOR for m in means_m]
            bars2 = ax2.bar(months, means_m, color=colors_m, alpha=0.7, edgecolor="none")
            ax2.axhline(0, color=GRID_COLOR, linewidth=0.5)
            ax2.set_title("Month of Year", color=TEXT_COLOR)
            ax2.set_ylabel("Mean Return (%)")
            ax2.grid(True, alpha=0.2, axis="y")
            ax2.tick_params(axis="x", rotation=45)
            # Mark significant months
            for i, m in enumerate(months):
                if month_stats[m].get("significant"):
                    ax2.text(i, means_m[i], "★", ha="center", va="bottom" if means_m[i] > 0 else "top",
                            color=ACCENT2, fontsize=12, fontweight="bold")

        plt.tight_layout()
        save_chart(fig, chart_path)

    # Format text
    text = f"""📅 <b>Seasonal Analysis: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 {n} observasi, {timeframe}
Today: {current_dow}, {current_month}

📆 <b>Day of Week:</b>
"""
    # Sort by mean return
    for day in sorted(dow_stats.keys(), key=lambda d: dow_stats[d]["mean_return"], reverse=True):
        ds = dow_stats[day]
        emoji = "🟢" if ds["mean_return"] > 0 else "🔴"
        sig = " ★" if ds["significant"] else ""
        text += f"  {emoji} {day}: {ds['mean_return']*100:+.3f}% (win {ds['win_rate']*100:.0f}%, n={ds['count']}){sig}\n"

    text += "\n📅 <b>Month of Year:</b>\n"
    for month in sorted(month_stats.keys(), key=lambda m: month_stats[m]["mean_return"], reverse=True):
        ms = month_stats[month]
        emoji = "🟢" if ms["mean_return"] > 0 else "🔴"
        sig = " ★" if ms["significant"] else ""
        text += f"  {emoji} {month}: {ms['mean_return']*100:+.3f}% (win {ms['win_rate']*100:.0f}%, n={ms['count']}){sig}\n"

    text += "\n★ = statistically significant (p≤0.05)"

    # Current context advice
    if current_dow in dow_stats:
        ds = dow_stats[current_dow]
        if ds["mean_return"] > 0:
            text += f"\n\n💡 <b>{current_dow} historically bullish</b> ({ds['mean_return']*100:+.3f}%, win {ds['win_rate']*100:.0f}%)"
        else:
            text += f"\n\n💡 <b>{current_dow} historically bearish</b> ({ds['mean_return']*100:+.3f}%, win {ds['win_rate']*100:.0f}%)"

    return output("seasonal", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# Helpers for GARCH (percentileofscore)
# ===========================================================================
def sp_percentileofscore(a, score):
    from scipy.stats import percentileofscore
    return percentileofscore(a, score)


# ===========================================================================
# MODE: COINTEGRATION — Pairs Trading (Engle-Granger)
# ===========================================================================

def compute_cointegration(df, symbol, timeframe, params, multi_asset, chart_path=None):
    from statsmodels.tsa.stattools import coint, adfuller

    close_primary = df["Close"]
    results_all = {}
    best_pair = None
    best_p = 1.0

    for sym, bars in (multi_asset or {}).items():
        if not bars or len(bars) < 50:
            continue
        closes = pd.Series(
            [float(b["close"]) for b in bars],
            index=[pd.Timestamp(b["date"]) for b in bars],
            name=sym,
        ).sort_index()

        # Align dates
        aligned = pd.DataFrame({symbol: close_primary, sym: closes}).dropna()
        if len(aligned) < 50:
            continue

        try:
            score, p_value, _ = coint(aligned[symbol], aligned[sym])
            is_coint = p_value < 0.05

            # Compute spread and half-life if cointegrated
            spread_info = {}
            if is_coint:
                from statsmodels.regression.linear_model import OLS
                import statsmodels.api as sm
                X = sm.add_constant(aligned[sym])
                model = OLS(aligned[symbol], X).fit()
                hedge_ratio = float(model.params.iloc[1])
                spread = aligned[symbol] - hedge_ratio * aligned[sym]
                spread_mean = spread.mean()
                spread_std = spread.std()
                zscore_spread = (spread - spread_mean) / spread_std if spread_std > 0 else spread * 0

                # Half-life
                lag = spread.shift(1).dropna()
                delta = spread.diff().dropna()
                idx = lag.index.intersection(delta.index)
                half_life = None
                if len(idx) > 10:
                    try:
                        hl_model = OLS(delta.loc[idx], lag.loc[idx]).fit()
                        if hl_model.params.iloc[0] < 0:
                            half_life = float(-np.log(2) / hl_model.params.iloc[0])
                    except Exception:
                        pass

                spread_info = {
                    "hedge_ratio": safe_float(hedge_ratio),
                    "current_zscore": safe_float(zscore_spread.iloc[-1]) if len(zscore_spread) > 0 else None,
                    "half_life": safe_float(half_life),
                    "spread_series": zscore_spread.values.tolist()[-100:],  # last 100 for chart
                    "dates": [d.strftime("%Y-%m-%d") for d in zscore_spread.index[-100:]],
                }

            results_all[sym] = {
                "coint_stat": safe_float(score),
                "p_value": safe_float(p_value),
                "is_cointegrated": is_coint,
                **spread_info,
            }

            if p_value < best_p:
                best_p = p_value
                best_pair = sym

        except Exception:
            continue

    if not results_all:
        return output("cointegration", symbol, False, {}, "", error="Tidak cukup data multi-aset untuk cointegration test")

    result = {"pairs": results_all, "best_pair": best_pair}

    # Chart: best pair spread Z-score
    if chart_path and best_pair and results_all[best_pair].get("spread_series"):
        fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(14, 7), gridspec_kw={"height_ratios": [1, 1]})
        fig.suptitle(f"Cointegration: {symbol} vs {best_pair} — {timeframe}",
                    color=TEXT_COLOR, fontsize=13, fontweight="bold")

        pair_data = results_all[best_pair]
        dates = pd.to_datetime(pair_data["dates"])
        zscore = pair_data["spread_series"]

        # Price comparison (normalized)
        aligned = pd.DataFrame({symbol: close_primary}).dropna()
        norm_sym = close_primary / close_primary.iloc[0]
        ax1.plot(close_primary.index[-len(norm_sym):], norm_sym, color=ACCENT1, linewidth=1, label=symbol)

        pair_closes = pd.Series(
            [float(b["close"]) for b in multi_asset[best_pair]],
            index=[pd.Timestamp(b["date"]) for b in multi_asset[best_pair]],
        ).sort_index()
        norm_pair = pair_closes / pair_closes.iloc[0]
        ax1.plot(pair_closes.index[-len(norm_pair):], norm_pair, color=ACCENT4, linewidth=1, label=best_pair)
        ax1.set_ylabel("Normalized Price")
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # Spread Z-score
        ax2.fill_between(dates, 0, zscore, where=[z > 0 for z in zscore], alpha=0.3, color=DOWN_COLOR)
        ax2.fill_between(dates, 0, zscore, where=[z <= 0 for z in zscore], alpha=0.3, color=UP_COLOR)
        ax2.plot(dates, zscore, color=TEXT_COLOR, linewidth=0.8)
        ax2.axhline(2, color=DOWN_COLOR, linestyle="--", linewidth=0.7, alpha=0.7)
        ax2.axhline(-2, color=UP_COLOR, linestyle="--", linewidth=0.7, alpha=0.7)
        ax2.axhline(0, color=GRID_COLOR, linewidth=0.5)
        ax2.set_ylabel("Spread Z-Score")
        ax2.set_ylim(-3.5, 3.5)
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""🔗 <b>Cointegration Analysis: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
"Pair mana yang bergerak bareng long-term?"

"""
    for sym, data in sorted(results_all.items(), key=lambda x: x[1]["p_value"]):
        emoji = "✅" if data["is_cointegrated"] else "❌"
        text += f"  {emoji} {symbol}↔{sym}: p={data['p_value']:.4f}\n"
        if data["is_cointegrated"]:
            hr = data.get("hedge_ratio", 0)
            zs = data.get("current_zscore")
            hl = data.get("half_life")
            text += f"    Hedge ratio: {hr:.4f}\n"
            if zs is not None:
                text += f"    Current Z-score: {zs:+.2f}\n"
            if hl is not None:
                text += f"    Half-life: {hl:.1f} bar\n"

    if best_pair and results_all[best_pair]["is_cointegrated"]:
        zs = results_all[best_pair].get("current_zscore", 0)
        if zs is not None and abs(zs) > 2:
            direction = "LONG spread" if zs < -2 else "SHORT spread"
            text += f"\n💡 <b>Signal: {direction}</b> ({symbol} vs {best_pair})"
            text += f"\n  Spread Z-score {zs:+.2f} — extreme, mean-revert expected"
        else:
            text += f"\n💡 Best pair: {symbol}↔{best_pair} — no extreme signal now"
    else:
        text += "\n⚠️ No strongly cointegrated pairs found."

    return output("cointegration", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: PCA — Principal Component Analysis (Factor Decomposition)
# ===========================================================================

def compute_pca(df, symbol, timeframe, params, multi_asset, chart_path=None):
    from sklearn.decomposition import PCA
    from sklearn.preprocessing import StandardScaler

    # Build multi-asset return matrix
    all_returns = {}
    returns_primary = compute_returns(df)
    all_returns[symbol] = returns_primary

    for sym, bars in (multi_asset or {}).items():
        if not bars or len(bars) < 50:
            continue
        closes = pd.Series(
            [float(b["close"]) for b in bars],
            index=[pd.Timestamp(b["date"]) for b in bars],
        ).sort_index()
        ret = np.log(closes / closes.shift(1)).dropna()
        if len(ret) > 20:
            all_returns[sym] = ret

    if len(all_returns) < 3:
        return output("pca", symbol, False, {}, "", error="Minimal 3 aset untuk PCA")

    ret_df = pd.DataFrame(all_returns).dropna()
    lookback = min(params.get("lookback", 252), len(ret_df))
    ret_df = ret_df.iloc[-lookback:]

    if len(ret_df) < 30:
        return output("pca", symbol, False, {}, "", error="Insufficient overlapping data for PCA")

    symbols = list(ret_df.columns)
    n_components = min(len(symbols), 5)

    # Standardize
    scaler = StandardScaler()
    scaled = scaler.fit_transform(ret_df)

    # PCA
    pca = PCA(n_components=n_components)
    pca.fit(scaled)

    explained_var = pca.explained_variance_ratio_
    cumulative_var = np.cumsum(explained_var)
    loadings = pd.DataFrame(
        pca.components_.T,
        columns=[f"PC{i+1}" for i in range(n_components)],
        index=symbols,
    )

    # How much of primary symbol is explained by each PC
    primary_loadings = loadings.loc[symbol] if symbol in loadings.index else None

    # Interpret top factors
    factor_names = []
    for i in range(min(3, n_components)):
        pc = loadings[f"PC{i+1}"]
        top_pos = pc.nlargest(3).index.tolist()
        top_neg = pc.nsmallest(3).index.tolist()
        factor_names.append({
            "pc": i + 1,
            "var_explained": safe_float(explained_var[i]),
            "top_positive": top_pos,
            "top_negative": top_neg,
        })

    result = {
        "n_components": n_components,
        "explained_variance": [safe_float(v) for v in explained_var],
        "cumulative_variance": [safe_float(v) for v in cumulative_var],
        "factors": factor_names,
        "primary_loadings": {f"PC{i+1}": safe_float(v) for i, v in enumerate(primary_loadings)} if primary_loadings is not None else {},
    }

    # Chart: scree plot + biplot
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6))
        fig.suptitle(f"PCA Factor Decomposition — {timeframe} ({len(ret_df)}d)",
                    color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Scree plot
        x = range(1, n_components + 1)
        ax1.bar(x, explained_var * 100, color=ACCENT1, alpha=0.7, label="Individual")
        ax1.plot(x, cumulative_var * 100, color=ACCENT2, marker="o", linewidth=2, label="Cumulative")
        ax1.axhline(80, color=DOWN_COLOR, linestyle="--", linewidth=0.7, alpha=0.5, label="80% threshold")
        ax1.set_xlabel("Principal Component")
        ax1.set_ylabel("Variance Explained (%)")
        ax1.set_title("Scree Plot", color=TEXT_COLOR)
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # Loading heatmap for top 3 PCs
        import seaborn as sns
        load_display = loadings.iloc[:, :min(3, n_components)]
        sns.heatmap(load_display, cmap="RdYlGn", center=0, annot=True, fmt=".2f",
                   linewidths=0.5, linecolor=GRID_COLOR, ax=ax2,
                   annot_kws={"size": 7, "color": TEXT_COLOR},
                   cbar_kws={"shrink": 0.8})
        ax2.set_title("Factor Loadings", color=TEXT_COLOR)
        ax2.tick_params(colors=TEXT_COLOR)

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""🧬 <b>PCA Factor Analysis: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 {len(symbols)} aset, {len(ret_df)} observasi

<b>Variance Explained:</b>
"""
    for i in range(min(3, n_components)):
        text += f"  PC{i+1}: {explained_var[i]*100:.1f}% (cum: {cumulative_var[i]*100:.1f}%)\n"

    # How many PCs for 80%?
    n_80 = int(np.argmax(cumulative_var >= 0.80)) + 1 if any(cumulative_var >= 0.80) else n_components
    text += f"\n📈 <b>{n_80} faktor menjelaskan {cumulative_var[min(n_80-1, len(cumulative_var)-1)]*100:.0f}% variance</b>\n"

    text += "\n<b>Factor Interpretations:</b>\n"
    for f in factor_names[:3]:
        text += f"\n  <b>PC{f['pc']} ({f['var_explained']*100:.1f}%):</b>\n"
        text += f"    Positif: {', '.join(f['top_positive'][:3])}\n"
        text += f"    Negatif: {', '.join(f['top_negative'][:3])}\n"

    if primary_loadings is not None:
        text += f"\n<b>{symbol} factor exposure:</b>\n"
        for i in range(min(3, n_components)):
            v = primary_loadings.iloc[i]
            text += f"  PC{i+1}: {v:+.3f}\n"

    return output("pca", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: VAR — Vector Autoregression (Multi-Asset Forecast)
# ===========================================================================

def compute_var(df, symbol, timeframe, params, multi_asset, chart_path=None):
    from statsmodels.tsa.api import VAR as VARModel

    # Build multi-asset return matrix (select related assets)
    all_returns = {}
    returns_primary = compute_returns(df)
    all_returns[symbol] = returns_primary

    for sym, bars in (multi_asset or {}).items():
        if not bars or len(bars) < 100:
            continue
        closes = pd.Series(
            [float(b["close"]) for b in bars],
            index=[pd.Timestamp(b["date"]) for b in bars],
        ).sort_index()
        ret = np.log(closes / closes.shift(1)).dropna()
        if len(ret) > 50:
            all_returns[sym] = ret

    if len(all_returns) < 2:
        return output("var", symbol, False, {}, "", error="Minimal 2 aset untuk VAR model")

    # Limit to 8 most correlated assets (performance)
    ret_df = pd.DataFrame(all_returns).dropna()
    if len(ret_df.columns) > 8:
        corr_with_primary = ret_df.corr()[symbol].abs().sort_values(ascending=False)
        top_syms = corr_with_primary.index[:8].tolist()
        if symbol not in top_syms:
            top_syms = [symbol] + top_syms[:7]
        ret_df = ret_df[top_syms]

    lookback = min(params.get("lookback", 252), len(ret_df))
    ret_df = ret_df.iloc[-lookback:]

    if len(ret_df) < 50:
        return output("var", symbol, False, {}, "", error="Insufficient data for VAR")

    # Fit VAR — with regularization fallback for near-singular matrices
    try:
        # Drop highly correlated columns to improve matrix conditioning
        corr_matrix = ret_df.corr().abs()
        upper = corr_matrix.where(np.triu(np.ones(corr_matrix.shape), k=1).astype(bool))
        to_drop = [c for c in upper.columns if any(upper[c] > 0.97)]
        if symbol in to_drop:
            to_drop.remove(symbol)
        if to_drop:
            ret_df = ret_df.drop(columns=to_drop)

        if len(ret_df.columns) < 2:
            return output("var", symbol, False, {}, "", error="Aset terlalu berkorelasi untuk VAR")

        model = VARModel(ret_df)
        # Auto-select lag (max 5 for stability)
        try:
            lag_order = model.select_order(maxlags=min(5, len(ret_df) // 10))
            best_lag = lag_order.aic
        except Exception:
            best_lag = 1
        if best_lag < 1:
            best_lag = 1

        results = model.fit(best_lag)
    except Exception as e:
        # Try simpler fallback: fewer assets, lag=1
        try:
            simple_df = ret_df[[symbol, ret_df.columns[1]]].dropna()
            results = VARModel(simple_df).fit(1)
            ret_df = simple_df
            best_lag = 1
        except Exception as e2:
            return output("var", symbol, False, {}, "", error=f"VAR fitting failed: {str(e2)}")

    # Forecast
    horizon = params.get("forecast_horizon", 5)
    forecast = results.forecast(ret_df.values[-best_lag:], steps=horizon)
    fc_df = pd.DataFrame(forecast, columns=ret_df.columns)

    # Impulse Response Function (how shocks in one asset affect others)
    try:
        irf = results.irf(10)
        irf_data = {}
        sym_idx = list(ret_df.columns).index(symbol)
        for i, sym in enumerate(ret_df.columns):
            if sym == symbol:
                continue
            # Response of primary symbol to shock in sym
            response = irf.irfs[:, sym_idx, i].tolist()
            irf_data[sym] = [safe_float(v) for v in response]
    except Exception:
        irf_data = {}

    # Granger-like: which assets have significant coefficients for primary
    sig_predictors = []
    try:
        coef_matrix = results.coefs  # shape: (lag, n_vars, n_vars)
        sym_list = list(ret_df.columns)
        target_idx = sym_list.index(symbol)
        for i, sym in enumerate(sym_list):
            if sym == symbol:
                continue
            total_coef = sum(abs(coef_matrix[lag][target_idx][i]) for lag in range(best_lag))
            if total_coef > 0.1:
                sig_predictors.append(sym)
    except Exception:
        pass

    result = {
        "lag_order": best_lag,
        "n_variables": len(ret_df.columns),
        "variables": list(ret_df.columns),
        "forecast": {sym: [safe_float(fc_df[sym].iloc[i]) for i in range(horizon)] for sym in ret_df.columns},
        "significant_predictors": sig_predictors,
    }

    # Chart: IRF for primary symbol
    if chart_path and irf_data:
        n_plots = min(len(irf_data), 6)
        fig, axes = plt.subplots(2, 3, figsize=(14, 8))
        fig.suptitle(f"VAR Impulse Response: shock → {symbol} — {timeframe}",
                    color=TEXT_COLOR, fontsize=13, fontweight="bold")
        axes = axes.flatten()

        for idx, (shock_sym, response) in enumerate(list(irf_data.items())[:n_plots]):
            ax = axes[idx]
            x = range(len(response))
            ax.bar(x, response, color=[UP_COLOR if v >= 0 else DOWN_COLOR for v in response], alpha=0.7)
            ax.axhline(0, color=GRID_COLOR, linewidth=0.5)
            ax.set_title(f"Shock: {shock_sym}", color=TEXT_COLOR, fontsize=9)
            ax.set_xlabel("Periods")
            ax.grid(True, alpha=0.2)

        # Hide unused axes
        for idx in range(n_plots, len(axes)):
            axes[idx].set_visible(False)

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""🌐 <b>VAR Model: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
Model: VAR({best_lag}) — {len(ret_df.columns)} variabel

<b>Variabel:</b> {', '.join(ret_df.columns)}

📊 <b>Forecast ({horizon} bar):</b>
"""
    for sym in [symbol] + [s for s in ret_df.columns if s != symbol]:
        fc_values = [fc_df[sym].iloc[i] for i in range(horizon)]
        direction = "+" if sum(fc_values) > 0 else "-"
        text += f"  {sym}: {direction}{abs(sum(fc_values))*100:.3f}% cum\n"

    if sig_predictors:
        text += f"\n⚡ <b>Significant predictors for {symbol}:</b>\n"
        for sp in sig_predictors:
            text += f"  → {sp}\n"

    if irf_data:
        text += f"\n📈 <b>Impulse Response (strongest):</b>\n"
        strongest = sorted(irf_data.items(), key=lambda x: max(abs(v) for v in x[1]), reverse=True)[:3]
        for shock_sym, response in strongest:
            peak = max(response, key=abs)
            peak_idx = response.index(peak)
            text += f"  {shock_sym} shock → {symbol}: peak {peak*100:+.4f}% at period {peak_idx}\n"

    return output("var", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: RISK — Portfolio VaR + CVaR + Tail Risk
# ===========================================================================

def compute_risk(df, symbol, timeframe, params, chart_path=None):
    from scipy import stats as sp_stats

    returns = compute_returns(df)
    n = len(returns)
    if n < 50:
        return output("risk", symbol, False, {}, "", error="Minimal 50 bar untuk risk analysis")

    ann = annualization_factor(timeframe)
    confidence = params.get("confidence_level", 0.95)

    # Historical VaR & CVaR
    alpha = 1 - confidence
    var_hist = float(np.percentile(returns, alpha * 100))
    cvar_hist = float(returns[returns <= var_hist].mean()) if len(returns[returns <= var_hist]) > 0 else var_hist

    # Parametric VaR (assuming normal)
    var_param = float(returns.mean() + sp_stats.norm.ppf(alpha) * returns.std())
    cvar_param = float(returns.mean() - returns.std() * sp_stats.norm.pdf(sp_stats.norm.ppf(alpha)) / alpha)

    # Cornish-Fisher VaR (adjusts for skewness and kurtosis)
    skew = float(sp_stats.skew(returns))
    kurt = float(sp_stats.kurtosis(returns, fisher=True))
    z = sp_stats.norm.ppf(alpha)
    z_cf = z + (z**2 - 1) * skew / 6 + (z**3 - 3*z) * kurt / 24 - (2*z**3 - 5*z) * skew**2 / 36
    var_cf = float(returns.mean() + z_cf * returns.std())

    # Maximum Drawdown
    cum_returns = (1 + returns).cumprod()
    running_max = cum_returns.cummax()
    drawdown = (cum_returns - running_max) / running_max
    max_dd = float(drawdown.min())
    max_dd_end = drawdown.idxmin()

    # Tail ratio (right tail vs left tail)
    tail_ratio = float(abs(np.percentile(returns, 95)) / abs(np.percentile(returns, 5))) if abs(np.percentile(returns, 5)) > 0 else 1

    # Stress test: worst N-day scenarios
    worst_1d = float(returns.min())
    worst_5d = float(returns.rolling(5).sum().min()) if n >= 5 else worst_1d
    worst_20d = float(returns.rolling(20).sum().min()) if n >= 20 else worst_5d

    # Dollar amounts (assuming $10,000 portfolio)
    portfolio = 10000
    dollar_var = abs(var_hist) * portfolio
    dollar_cvar = abs(cvar_hist) * portfolio
    dollar_mdd = abs(max_dd) * portfolio

    result = {
        "confidence": confidence,
        "var_historical": safe_float(var_hist),
        "cvar_historical": safe_float(cvar_hist),
        "var_parametric": safe_float(var_param),
        "var_cornish_fisher": safe_float(var_cf),
        "max_drawdown": safe_float(max_dd),
        "tail_ratio": safe_float(tail_ratio),
        "worst_1d": safe_float(worst_1d),
        "worst_5d": safe_float(worst_5d),
        "worst_20d": safe_float(worst_20d),
        "skewness": safe_float(skew),
        "kurtosis": safe_float(kurt),
        "dollar_var": safe_float(dollar_var),
        "dollar_cvar": safe_float(dollar_cvar),
        "dollar_mdd": safe_float(dollar_mdd),
    }

    # Chart: drawdown + return distribution with VaR markers
    if chart_path:
        fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(14, 8), gridspec_kw={"height_ratios": [1, 1]})
        fig.suptitle(f"{symbol} — Risk Analysis — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Drawdown chart
        ax1.fill_between(drawdown.index, 0, drawdown * 100, color=DOWN_COLOR, alpha=0.4)
        ax1.plot(drawdown.index, drawdown * 100, color=DOWN_COLOR, linewidth=0.8)
        ax1.axhline(max_dd * 100, color=ACCENT2, linestyle="--", linewidth=0.8,
                   label=f"Max DD: {max_dd*100:.1f}%")
        ax1.set_ylabel("Drawdown (%)")
        ax1.set_title("Drawdown History", color=TEXT_COLOR)
        ax1.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax1.grid(True, alpha=0.3)

        # Return distribution with VaR
        ax2.hist(returns, bins=50, color=ACCENT1, alpha=0.6, edgecolor="none", density=True)
        ax2.axvline(var_hist, color=DOWN_COLOR, linestyle="-", linewidth=2,
                   label=f"VaR {confidence*100:.0f}%: {var_hist*100:.3f}%")
        ax2.axvline(cvar_hist, color=ACCENT4, linestyle="--", linewidth=2,
                   label=f"CVaR: {cvar_hist*100:.3f}%")
        ax2.axvline(var_cf, color=ACCENT3, linestyle=":", linewidth=1.5,
                   label=f"CF-VaR: {var_cf*100:.3f}%")
        ax2.set_title("Return Distribution + VaR", color=TEXT_COLOR)
        ax2.set_xlabel("Return")
        ax2.legend(fontsize=7, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax2.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    # Risk rating
    if abs(max_dd) > 0.20:
        risk_rating = "🔴 HIGH RISK"
    elif abs(max_dd) > 0.10:
        risk_rating = "🟡 MODERATE RISK"
    else:
        risk_rating = "🟢 LOW RISK"

    text = f"""⚠️ <b>Risk Analysis: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
Overall: <b>{risk_rating}</b>

📊 <b>Value at Risk ({confidence*100:.0f}% confidence):</b>
  Historical VaR: {var_hist*100:.3f}%/bar
  Parametric VaR: {var_param*100:.3f}%/bar
  Cornish-Fisher VaR: {var_cf*100:.3f}%
  CVaR (Expected Shortfall): {cvar_hist*100:.3f}%/bar

💰 <b>Dollar Impact ($10,000 portfolio):</b>
  Daily VaR: ${dollar_var:.0f}
  Daily CVaR: ${dollar_cvar:.0f}
  Max Drawdown: ${dollar_mdd:.0f}

📉 <b>Drawdown:</b>
  Max Drawdown: {max_dd*100:.1f}%
  Worst 1-day: {worst_1d*100:.3f}%
  Worst 5-day: {worst_5d*100:.3f}%
  Worst 20-day: {worst_20d*100:.3f}%

📐 <b>Tail Risk:</b>
  Skewness: {skew:.2f} {"(left tail risk ⚠️)" if skew < -0.3 else "(simetris)" if abs(skew) < 0.3 else "(right tail)"}
  Kurtosis: {kurt:.2f} {"(fat tails ⚠️)" if kurt > 3 else "(normal tails)"}
  Tail Ratio: {tail_ratio:.2f} {"(tails balanced)" if 0.8 < tail_ratio < 1.2 else "(asymmetric tails ⚠️)"}

💡 <b>Position Sizing Suggestion:</b>
  1% risk rule: max ${portfolio*0.01/abs(var_hist):.0f} position per trade
  2% risk rule: max ${portfolio*0.02/abs(var_hist):.0f} position per trade"""

    return output("risk", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: FULL — Comprehensive Quant Report (runs all models)
# ===========================================================================

def compute_full_report(df, symbol, timeframe, params, multi_asset, chart_path=None):
    """Run core models and synthesize into a single decision."""

    sub_results = {}
    errors = []

    # Run each sub-model (no charts for individual — we'll make a summary chart)
    try:
        sub_results["stats"] = compute_stats(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"stats: {e}")

    try:
        sub_results["garch"] = compute_garch(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"garch: {e}")

    try:
        sub_results["regime"] = compute_regime(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"regime: {e}")

    try:
        sub_results["meanrevert"] = compute_meanrevert(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"meanrevert: {e}")

    try:
        sub_results["seasonal"] = compute_seasonal(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"seasonal: {e}")

    try:
        sub_results["risk"] = compute_risk(df, symbol, timeframe, params)
    except Exception as e:
        errors.append(f"risk: {e}")

    if multi_asset:
        try:
            sub_results["correlation"] = compute_correlation(df, symbol, timeframe, params, multi_asset)
        except Exception as e:
            errors.append(f"correlation: {e}")

    # Synthesize decision
    signals = []
    confidence_scores = []

    # Regime signal
    if "regime" in sub_results and sub_results["regime"]["success"]:
        regime_result = sub_results["regime"]["result"]
        regime_idx = regime_result.get("current_regime", 1)
        regime_prob = regime_result.get("current_probability", 0.5)
        if regime_idx == 0:  # Bull
            signals.append(("Regime", "BULLISH", regime_prob))
            confidence_scores.append(regime_prob * 0.3)
        elif regime_idx == 2:  # Bear
            signals.append(("Regime", "BEARISH", regime_prob))
            confidence_scores.append(-regime_prob * 0.3)
        else:
            signals.append(("Regime", "NEUTRAL", regime_prob))

    # Seasonal signal
    if "seasonal" in sub_results and sub_results["seasonal"]["success"]:
        seasonal_result = sub_results["seasonal"]["result"]
        current_dow = seasonal_result.get("current_day", "")
        dow_data = seasonal_result.get("day_of_week", {})
        if current_dow in dow_data:
            dow_mean = dow_data[current_dow].get("mean_return", 0) or 0
            dow_wr = dow_data[current_dow].get("win_rate", 0.5) or 0.5
            if dow_mean > 0 and dow_wr > 0.55:
                signals.append(("Seasonal", "BULLISH", min(dow_wr, 1.0)))
                confidence_scores.append(0.1)
            elif dow_mean < 0 and dow_wr < 0.45:
                signals.append(("Seasonal", "BEARISH", min(1 - dow_wr, 1.0)))
                confidence_scores.append(-0.1)
            else:
                signals.append(("Seasonal", "NEUTRAL", 0.5))

    # Mean reversion signal
    if "meanrevert" in sub_results and sub_results["meanrevert"]["success"]:
        mr_result = sub_results["meanrevert"]["result"]
        zscore = mr_result.get("current_zscore", 0)
        if zscore is not None:
            if zscore > 2:
                signals.append(("Z-Score", "BEARISH (overbought)", min(abs(zscore)/4, 1)))
                confidence_scores.append(-0.2)
            elif zscore < -2:
                signals.append(("Z-Score", "BULLISH (oversold)", min(abs(zscore)/4, 1)))
                confidence_scores.append(0.2)
            else:
                signals.append(("Z-Score", "NEUTRAL", 0.5))

    # Vol regime (position sizing signal)
    vol_signal = "NORMAL"
    if "garch" in sub_results and sub_results["garch"]["success"]:
        garch_result = sub_results["garch"]["result"]
        vol_pct = garch_result.get("vol_percentile", 50)
        if vol_pct is not None:
            if vol_pct > 75:
                vol_signal = "REDUCE SIZE"
            elif vol_pct < 25:
                vol_signal = "FULL SIZE"
            else:
                vol_signal = "STANDARD SIZE"

    # Compute overall
    total_conf = sum(confidence_scores) if confidence_scores else 0
    if total_conf > 0.15:
        overall = "🟢 BULLISH"
        direction = "LONG"
    elif total_conf < -0.15:
        overall = "🔴 BEARISH"
        direction = "SHORT"
    else:
        overall = "🟡 NEUTRAL"
        direction = "FLAT"

    overall_pct = min(abs(total_conf) / 0.65 * 100, 100)

    result = {
        "overall_signal": direction,
        "overall_confidence": safe_float(overall_pct),
        "vol_signal": vol_signal,
        "signals": [{"source": s[0], "direction": s[1], "strength": safe_float(s[2])} for s in signals],
        "errors": errors,
        "models_run": len(sub_results),
    }

    # Chart: dashboard summary
    if chart_path:
        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle(f"{symbol} — Full Quant Report — {timeframe}",
                    color=TEXT_COLOR, fontsize=14, fontweight="bold")

        # 1. Price + regime coloring
        ax = axes[0, 0]
        close = df["Close"]
        ax.plot(close.index, close, color=TEXT_COLOR, linewidth=1)
        ax.set_title(f"Price ({overall})", color=TEXT_COLOR)
        ax.grid(True, alpha=0.2)

        # 2. Signal gauge
        ax = axes[0, 1]
        signal_labels = [s[0] for s in signals]
        signal_values = [s[2] if s[1].startswith("BULL") else -s[2] if s[1].startswith("BEAR") else 0 for s in signals]
        colors = [UP_COLOR if v > 0 else DOWN_COLOR if v < 0 else ACCENT2 for v in signal_values]
        if signal_labels:
            y_pos = range(len(signal_labels))
            ax.barh(y_pos, signal_values, color=colors, alpha=0.7)
            ax.set_yticks(y_pos)
            ax.set_yticklabels(signal_labels, fontsize=8)
            ax.axvline(0, color=GRID_COLOR, linewidth=0.5)
            ax.set_xlim(-1.1, 1.1)
        ax.set_title("Signal Breakdown", color=TEXT_COLOR)
        ax.grid(True, alpha=0.2, axis="x")

        # 3. Returns distribution
        ax = axes[1, 0]
        returns = compute_returns(df)
        ax.hist(returns, bins=40, color=ACCENT1, alpha=0.6, edgecolor="none", density=True)
        ax.set_title("Return Distribution", color=TEXT_COLOR)
        ax.grid(True, alpha=0.2)

        # 4. Rolling volatility
        ax = axes[1, 1]
        ann_f = annualization_factor(timeframe)
        rv = returns.rolling(20).std() * np.sqrt(ann_f) * 100
        ax.fill_between(rv.index, 0, rv, color=ACCENT3, alpha=0.3)
        ax.plot(rv.index, rv, color=ACCENT3, linewidth=1)
        ax.set_title("Rolling Volatility (20d)", color=TEXT_COLOR)
        ax.set_ylabel("Annualized Vol (%)")
        ax.grid(True, alpha=0.2)

        plt.tight_layout()
        save_chart(fig, chart_path)

    text = f"""📋 <b>FULL QUANT REPORT: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 {timeframe} | {len(sub_results)} model dijalankan

🎯 <b>SIGNAL: {overall} ({direction})</b>
  Confidence: {overall_pct:.0f}%
  Position: {vol_signal}

━━━ Signal Breakdown ━━━
"""
    for s in signals:
        emoji = "🟢" if "BULL" in s[1] else ("🔴" if "BEAR" in s[1] else "🟡")
        text += f"  {emoji} {s[0]}: {s[1]} ({s[2]*100:.0f}%)\n"

    # Key metrics from sub-results
    if "stats" in sub_results and sub_results["stats"]["success"]:
        sr = sub_results["stats"]["result"]
        text += f"""
━━━ Key Metrics ━━━
  Sharpe: {sr.get("sharpe", 0):.2f}
  Ann. Vol: {(sr.get("ann_volatility", 0) or 0)*100:.1f}%
  VaR (95%): {(sr.get("var_95", 0) or 0)*100:.3f}%"""

    if "garch" in sub_results and sub_results["garch"]["success"]:
        gr = sub_results["garch"]["result"]
        text += f"""
  GARCH Vol: {(gr.get("ann_current_vol", 0) or 0)*100:.1f}% ann
  Vol Regime: {gr.get("vol_regime", "N/A")}"""

    if "regime" in sub_results and sub_results["regime"]["success"]:
        rr = sub_results["regime"]["result"]
        text += f"""
  Market Regime: {rr.get("current_label", "N/A")} ({(rr.get("current_probability", 0) or 0)*100:.0f}%)"""

    if "meanrevert" in sub_results and sub_results["meanrevert"]["success"]:
        mr = sub_results["meanrevert"]["result"]
        text += f"""
  Hurst: {mr.get("hurst_exponent", 0):.3f}
  Z-Score: {(mr.get("current_zscore", 0) or 0):+.2f}"""

    if errors:
        text += f"\n\n⚠️ {len(errors)} model error(s): {', '.join(e.split(':')[0] for e in errors)}"

    return output("full", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# Dispatcher
# ===========================================================================

MODES = {
    "stats": lambda data, chart: compute_stats(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "garch": lambda data, chart: compute_garch(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "correlation": lambda data, chart: compute_correlation(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
    "regime": lambda data, chart: compute_regime(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "meanrevert": lambda data, chart: compute_meanrevert(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "granger": lambda data, chart: compute_granger(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
    "seasonal": lambda data, chart: compute_seasonal(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "cointegration": lambda data, chart: compute_cointegration(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
    "pca": lambda data, chart: compute_pca(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
    "var": lambda data, chart: compute_var(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
    "risk": lambda data, chart: compute_risk(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
    "full": lambda data, chart: compute_full_report(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}),
        data.get("multi_asset", {}), chart),
}


def main():
    if len(sys.argv) < 3:
        print("Usage: python3 quant_engine.py <input.json> <output.json> [chart.png]", file=sys.stderr)
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2]
    chart_path = sys.argv[3] if len(sys.argv) > 3 else None

    with open(input_path, "r") as f:
        data = json.load(f)

    mode = data.get("mode", "stats")
    symbol = data.get("symbol", "???")

    handler = MODES.get(mode)
    if handler is None:
        result = output(mode, symbol, False, {}, "", error=f"Unknown mode: {mode}")
    else:
        try:
            result = handler(data, chart_path)
        except Exception as e:
            result = output(mode, symbol, False, {}, "",
                          error=f"{type(e).__name__}: {str(e)}\n{traceback.format_exc()[-500:]}")

    with open(output_path, "w") as f:
        json.dump(result, f, indent=2, default=str)

    print(f"Quant engine: mode={mode}, symbol={symbol}, success={result['success']}")


if __name__ == "__main__":
    main()
