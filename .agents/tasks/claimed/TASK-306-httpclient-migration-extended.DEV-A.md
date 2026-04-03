Claimed by: Dev-A
Claimed at: 2026-04-03 WIB
Status: done
PR: feat/TASK-306-httpclient-migration-extended
Scope: Migrate 18 bare &http.Client{} usages to httpclient.New() factory across sec, imf, treasury, bis, cot, vix, price/eia, news/fed_rss, fed, marketdata/massive, and all macro/* clients. Also fixed 2 pre-existing compile errors from TASK-254 (duplicate levelDisplay and duplicate reset_onboard case).
