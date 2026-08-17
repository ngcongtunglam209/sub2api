-- 分销商套餐：一次性购买，按比例返还余额，并放开转售所需的资源。
--
-- 与 vip_tiers 刻意分开：VIP 是按消费**挣**来的忠诚度等级，会随消费升降、
-- 会过期；分销商套餐是**买**来的合作资格，含返还额度、域名配额、可转售的
-- 分组白名单。两者现在形状相似，但一个动一个不动，混在一张表里迟早要拆。
--
-- 为什么返还额度而不是长期充值折扣：折扣按每一笔充值永久生效，且会与分组
-- 倍率、VIP 倍率连乘，账面上看不出来；返还是一次性的、有上限的、看得见的。
--
-- 为什么不给分销商更低的 rate_multiplier：分销商本人几乎不消耗 token，
-- 便宜的费率落不到他们头上；真要落到他们客户头上，那是替客户降价、由我们
-- 买单，分销商一分钱不多赚。他们的利润来自转售加价，那部分完全在系统之外。

CREATE TABLE IF NOT EXISTS reseller_plans (
    id BIGSERIAL PRIMARY KEY,
    level INTEGER NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL,
    price DECIMAL(20, 2) NOT NULL,
    credit_rate DECIMAL(6, 4) NOT NULL DEFAULT 0.5,
    concurrency_bonus INTEGER NOT NULL DEFAULT 0,
    rpm_limit INTEGER NOT NULL DEFAULT 0,
    max_domains INTEGER NOT NULL DEFAULT 1,
    validity_days INTEGER NOT NULL DEFAULT 365,
    allowed_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE reseller_plans IS '分销商套餐配置；购买后由管理员指派给用户。';
COMMENT ON COLUMN reseller_plans.credit_rate IS '购买价中返还为余额的比例；0.6 表示付 100 返 60。';
COMMENT ON COLUMN reseller_plans.concurrency_bonus IS '累加到 users.concurrency 之上，与 VIP 同为相加语义。';
COMMENT ON COLUMN reseller_plans.max_domains IS '该等级可登记的自定义域名数（见 reseller_domains）。';
COMMENT ON COLUMN reseller_plans.allowed_group_ids IS '可生成兑换码的分组白名单；空数组 = 不限制。分销商只能选我们定义的分组，不能自定义费率。';

CREATE INDEX IF NOT EXISTS idx_reseller_plans_enabled_level
    ON reseller_plans (enabled, level);

-- 初始三级。数值可在后台改，这里给一套能直接上线的默认值。
--
-- concurrency_bonus 刻意压得低：整个池子的并发上限是「可用账号数 × 3」，
-- 账号被限流时可用数会掉到个位数。并发是当前最稀缺的资源，不是拿来慷慨
-- 派发的筹码。T3 的 +10 已经超过当前可用容量，可用账号涨到 12-15 之前
-- 不要开卖 T3。
--
-- credit_rate 随等级放宽（0.5 → 0.7），但要记住返还的额度是要拿真账号去
-- 兑现的：T3 返还 $280 ≈ 280 亿 token，需要约 33 个 Plus 账号的月产量。
-- 卖 T3 收到的 $400 里，有约 $115 是货款不是利润。
INSERT INTO reseller_plans (level, name, price, credit_rate, concurrency_bonus, rpm_limit, max_domains, validity_days)
VALUES
    (1, 'Reseller 1', 50, 0.5000, 2, 60, 1, 365),
    (2, 'Reseller 2', 150, 0.6000, 5, 150, 3, 365),
    (3, 'Reseller 3', 400, 0.7000, 10, 300, 10, 365)
ON CONFLICT (level) DO NOTHING;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS reseller_plan_id BIGINT,
    ADD COLUMN IF NOT EXISTS reseller_plan_expires_at TIMESTAMPTZ;

COMMENT ON COLUMN users.reseller_plan_id IS '当前分销商套餐；NULL = 不是分销商。套餐被删除后按非分销商处理。';
COMMENT ON COLUMN users.reseller_plan_expires_at IS '套餐到期时间；到期后资源回落，域名与兑换码权限一并失效。';

-- 到期扫描只需要带套餐的行。
CREATE INDEX IF NOT EXISTS idx_users_reseller_plan_expires_at
    ON users (reseller_plan_expires_at)
    WHERE reseller_plan_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_reseller_plan_id
    ON users (reseller_plan_id)
    WHERE reseller_plan_id IS NOT NULL;
