#!/usr/bin/env python3
"""
Quant Engine — Econometric/Statistical Analysis for Trading.

Usage: python3 quant_engine.py <input.json> <output.json> [chart_output.png]

Modes: stats, garch, correlation, regime, arima, granger, meanrevert, pca, cointegration, var, risk, full

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
    text = f"""🔗 <b>Correlation Matrix: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 {len(ret_df)} observasi, {timeframe}

"""
    # Matrix display
    header = "       " + "  ".join(f"{s:>6}" for s in symbols)
    text += f"<code>{header}\n"
    for s1 in symbols:
        row = f"{s1:>6} "
        for s2 in symbols:
            v = corr.loc[s1, s2]
            row += f" {v:+.2f} "
        text += row + "\n"
    text += "</code>\n"

    # Key insights
    text += "\n🔑 <b>Key Insights:</b>\n"
    for sym, rc in rolling_corrs.items():
        curr = rc["current"]
        if curr is None:
            continue
        strength = "sangat kuat" if abs(curr) > 0.7 else ("moderate" if abs(curr) > 0.4 else "lemah")
        direction = "positif" if curr > 0 else "negatif"
        text += f"  {symbol}↔{sym}: {curr:+.2f} ({strength} {direction})\n"
        if rc.get("prev_30d") is not None:
            delta = curr - rc["prev_30d"]
            trend = "strengthening" if abs(curr) > abs(rc["prev_30d"]) else "weakening"
            text += f"    30d change: {delta:+.2f} ({trend})\n"

    return output("correlation", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# MODE: REGIME — Hidden Markov Model
# ===========================================================================

def compute_regime(df, symbol, timeframe, params, chart_path=None):
    from hmmlearn.hmm import GaussianHMM

    returns = compute_returns(df)
    n = len(returns)
    if n < 60:
        return output("regime", symbol, False, {}, "", error="Minimal 60 bar untuk regime detection")

    n_regimes = params.get("n_regimes", 3)
    ann = annualization_factor(timeframe)

    # Features: returns + absolute returns (vol proxy)
    features = np.column_stack([returns.values, np.abs(returns.values)])

    # Fit HMM
    best_model = None
    best_score = -np.inf
    for _ in range(10):  # multiple restarts for stability
        try:
            model = GaussianHMM(n_components=n_regimes, covariance_type="full",
                              n_iter=200, random_state=np.random.randint(10000),
                              tol=1e-4)
            model.fit(features)
            score = model.score(features)
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

    # Transition matrix
    trans_mat = best_model.transmat_
    # Reorder transition matrix by our sorting
    reorder = [ri["id"] for ri in regime_info]
    trans_reordered = trans_mat[np.ix_(reorder, reorder)]

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
        adf_interp = "✅ Stationary (p<0.01) — mean reversion kuat"
    elif adf_p < 0.05:
        adf_interp = "✅ Stationary (p<0.05) — mean reversion moderate"
    elif adf_p < 0.10:
        adf_interp = "⚠️ Marginal (p<0.10) — weak evidence"
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
    text = f"""🔄 <b>Mean Reversion Test: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 <b>ADF Test (Augmented Dickey-Fuller):</b>
  Price: stat={adf_stat:.3f}, p={adf_p:.4f}
  → {adf_interp}
  Returns: p={adf_ret_p:.4f} {"✅ stationary" if adf_ret_p < 0.05 else "❌"}

📈 <b>Hurst Exponent: {hurst:.3f}</b>
  → {hurst_interp}
  (0.5=random walk, <0.5=mean-revert, >0.5=trending)

📉 <b>Z-Score (current): {z_current:+.2f}</b>
  {"🔴 Overbought (>2σ) — reversal probability tinggi" if z_current > 2 else "🟢 Oversold (<-2σ) — bounce probability tinggi" if z_current < -2 else "🟡 Normal range"}"""

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
# MODE: ARIMA — Forecast
# ===========================================================================

def compute_arima(df, symbol, timeframe, params, chart_path=None):
    from statsmodels.tsa.arima.model import ARIMA

    close = df["Close"]
    n = len(close)
    if n < 50:
        return output("arima", symbol, False, {}, "", error="Minimal 50 bar untuk ARIMA forecast")

    horizon = params.get("forecast_horizon", 5)

    # Auto-select ARIMA order (simplified: try common orders)
    best_aic = np.inf
    best_order = (1, 1, 1)
    best_res = None

    orders = [(1,1,0), (0,1,1), (1,1,1), (2,1,1), (1,1,2), (2,1,2)]
    for order in orders:
        try:
            model = ARIMA(close, order=order)
            res = model.fit()
            if res.aic < best_aic:
                best_aic = res.aic
                best_order = order
                best_res = res
        except Exception:
            continue

    if best_res is None:
        return output("arima", symbol, False, {}, "", error="ARIMA fitting gagal")

    # Forecast
    fcast = best_res.get_forecast(steps=horizon)
    fc_mean = fcast.predicted_mean
    fc_ci = fcast.conf_int(alpha=0.05)

    result = {
        "order": list(best_order),
        "aic": safe_float(best_aic),
        "forecast": [safe_float(v) for v in fc_mean.values],
        "conf_lower": [safe_float(v) for v in fc_ci.iloc[:, 0].values],
        "conf_upper": [safe_float(v) for v in fc_ci.iloc[:, 1].values],
        "current_price": safe_float(close.iloc[-1]),
    }

    # Expected return
    expected_return = (fc_mean.iloc[-1] - close.iloc[-1]) / close.iloc[-1]

    # Chart
    if chart_path:
        fig, ax = plt.subplots(figsize=(14, 7))
        fig.suptitle(f"{symbol} — ARIMA{best_order} Forecast — {timeframe}", color=TEXT_COLOR, fontsize=13, fontweight="bold")

        # Last 60 bars
        recent = close.iloc[-60:]
        ax.plot(recent.index, recent, color=ACCENT1, linewidth=1.2, label="Price")

        # Forecast
        last_date = close.index[-1]
        fc_dates = pd.date_range(start=last_date, periods=horizon+1, freq="B")[1:]
        ax.plot(fc_dates, fc_mean.values, color=ACCENT2, linewidth=2, linestyle="--", label="Forecast", marker="o", markersize=4)
        ax.fill_between(fc_dates, fc_ci.iloc[:, 0].values, fc_ci.iloc[:, 1].values,
                        alpha=0.15, color=ACCENT2, label="95% CI")

        ax.axvline(last_date, color=GRID_COLOR, linestyle=":", linewidth=0.8)
        ax.set_ylabel("Price")
        ax.legend(fontsize=8, facecolor=BG_COLOR, edgecolor=GRID_COLOR)
        ax.grid(True, alpha=0.3)

        plt.tight_layout()
        save_chart(fig, chart_path)

    direction = "📈 NAIK" if expected_return > 0.001 else ("📉 TURUN" if expected_return < -0.001 else "➡️ FLAT")
    text = f"""📉 <b>ARIMA Forecast: {symbol}</b>
━━━━━━━━━━━━━━━━━━━━━━━━━━
Model: ARIMA{best_order} (AIC: {best_aic:.1f})

📊 <b>Forecast ({horizon} bar):</b>
  Current: {close.iloc[-1]:.4f}
"""
    for i in range(horizon):
        text += f"  Bar +{i+1}: {fc_mean.iloc[i]:.4f} [{fc_ci.iloc[i, 0]:.4f} — {fc_ci.iloc[i, 1]:.4f}]\n"

    text += f"""
🎯 <b>Expected: {direction} {expected_return*100:+.2f}%</b>

⚠️ ARIMA limitations: linear model, poor for volatility regimes.
Best combined with regime detection & vol models."""

    return output("arima", symbol, True, result, text, chart_path=chart_path or "")


# ===========================================================================
# Helpers for GARCH (percentileofscore)
# ===========================================================================
def sp_percentileofscore(a, score):
    from scipy.stats import percentileofscore
    return percentileofscore(a, score)


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
    "arima": lambda data, chart: compute_arima(
        bars_to_df(data["bars"]), data["symbol"], data["timeframe"], data.get("params", {}), chart),
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
