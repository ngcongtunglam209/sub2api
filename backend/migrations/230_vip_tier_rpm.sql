-- VIP 等级除并发外再赠送 RPM，并允许最高档不限量。
--
-- rpm_limit 是**加数**，与 concurrency 同语义：累加到 users.rpm_limit 之上，
-- 而不是覆盖。0 表示该等级不送 RPM。
--
-- 为什么不用 0 表示"不限量"：users.rpm_limit 与 groups.rpm_limit 里 0 已经
-- 表示不限量（见 billing_cache_service.go 的两层检查），但这里的数是加数而非
-- 上限，加 0 只能表示"不加"。同一列无法既表达"不加"又表达"无上限"，因此
-- 不限量另立布尔列。混用会让"VIP1 不送 RPM"被当成"VIP1 无上限"，把最便宜的
-- 等级变成最好的等级。
--
-- unlimited_concurrency 同理。注意 concurrency 列有 Positive() 约束、存不了 0，
-- 所以即使想用 0 当哨兵也做不到。
--
-- 两个布尔在鉴权快照里必须**最后**生效：VIP 之后还会叠加分销商套餐和加购项，
-- 若先把上限清成 0 再让它们相加，"无上限"会退化成那个加数。
--
-- 默认值让迁移无行为变化：既有等级 rpm_limit=0（不送）、两个 unlimited 均为
-- false，与迁移前完全一致。
ALTER TABLE vip_tiers
    ADD COLUMN IF NOT EXISTS rpm_limit INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unlimited_rpm BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS unlimited_concurrency BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN vip_tiers.rpm_limit IS '该等级额外赠送的 RPM，累加到 users.rpm_limit 之上；0 表示不赠送。';
COMMENT ON COLUMN vip_tiers.unlimited_rpm IS '该等级免除 RPM 上限；为真时忽略 rpm_limit，鉴权快照最后把上限清为 0（0=不限量）。';
COMMENT ON COLUMN vip_tiers.unlimited_concurrency IS '该等级免除并发上限；为真时忽略 concurrency，鉴权快照最后把上限清为 0（0=不限量）。';
