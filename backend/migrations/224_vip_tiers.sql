-- VIP 等级：按真实付费金额定级，给出计费折扣与并发提升。
--
-- 定级口径不用 users.total_recharged：该字段在 UpdateBalance 内累加，
-- 管理员手动加款、促销赠送、兑换码与联盟返佣提现都会算进去，而订阅订单
-- 因为不走余额反而一分不计。VIP 只认真实支付完成的订单。

CREATE TABLE IF NOT EXISTS vip_tiers (
    id BIGSERIAL PRIMARY KEY,
    level INTEGER NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL,
    min_spend_usd DECIMAL(20, 2) NOT NULL,
    rate_multiplier DECIMAL(6, 4) NOT NULL DEFAULT 1,
    concurrency INTEGER NOT NULL DEFAULT 5,
    grace_days INTEGER NOT NULL DEFAULT 60,
    badge_color VARCHAR(20) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE vip_tiers IS 'VIP 等级配置；VIP0 不建行，无匹配等级即 VIP0。';
COMMENT ON COLUMN vip_tiers.min_spend_usd IS '达标所需消费，与 payment_orders.amount 同为美元口径。';
COMMENT ON COLUMN vip_tiers.rate_multiplier IS '计费倍率，乘在分组倍率之上；0.95 表示只扣 95%。';
COMMENT ON COLUMN vip_tiers.grace_days IS '每笔订单完成后等级顺延的天数。';
COMMENT ON COLUMN vip_tiers.enabled IS '关掉后新用户不再进入该级，已在该级的用户保留到过期。';

CREATE INDEX IF NOT EXISTS idx_vip_tiers_enabled_min_spend
    ON vip_tiers (enabled, min_spend_usd);

-- 初始四级。数值可在后台改，这里只给一套可直接上线的默认值：
-- 每级门槛约为上一级的 3.75~5 倍，折扣越深门槛拉得越开，避免高档位过密。
INSERT INTO vip_tiers (level, name, min_spend_usd, rate_multiplier, concurrency, grace_days, badge_color)
VALUES
    (1, 'VIP1', 20, 0.95, 8, 60, '#9ca3af'),
    (2, 'VIP2', 100, 0.90, 12, 60, '#60a5fa'),
    (3, 'VIP3', 400, 0.82, 20, 60, '#a78bfa'),
    (4, 'VIP4', 1500, 0.70, 32, 60, '#f59e0b')
ON CONFLICT (level) DO NOTHING;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS total_paid_usd DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS vip_qualifying_spend DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS vip_tier_id BIGINT,
    ADD COLUMN IF NOT EXISTS vip_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS vip_tier_locked BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN users.total_paid_usd IS '真实支付完成订单的永久累计额，仅用于报表。';
COMMENT ON COLUMN users.vip_qualifying_spend IS '定级口径消费额；等级过期时清零，否则有效期形同虚设。';
COMMENT ON COLUMN users.vip_tier_id IS '当前等级；NULL 表示 VIP0。等级被删除后按 VIP0 处理。';
COMMENT ON COLUMN users.vip_tier_locked IS '管理员锁定的等级不随消费升降，也不过期。';

-- 过期扫描按 vip_expires_at 取到期用户，只有带等级的行需要进入索引。
CREATE INDEX IF NOT EXISTS idx_users_vip_expires_at
    ON users (vip_expires_at)
    WHERE vip_tier_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_vip_tier_id
    ON users (vip_tier_id)
    WHERE vip_tier_id IS NOT NULL;
