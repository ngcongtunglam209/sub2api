-- 自助加购：用户拿自己的余额买并发与 RPM，不走支付网关。
--
-- 钱早就通过充值进了余额，这里只是一次内部记账：扣余额 + 写额度，同一个
-- 事务。没有订单、没有回调、没有第三方状态机要对账。
--
-- 每个 (user_id, kind) 只有一行，重复购买是**续期**而不是叠加新行：
--
--   * 认证快照每次回源都要问"这个人买了多少并发"。一行 SELECT 回答得了，
--     一堆行就得 SUM，而这条路在鉴权热路径上。
--   * 上限是按"已持有的总量"卡的。一行就是总量本身，多行还要先聚合再比，
--     多一步就多一个漏判的机会。
--   * 用户看到的是一个数字和一个到期日，而不是七张分别到期的小票。
--
-- 代价：续期会把先买的那部分一起延后。这是明确的让利，且有上限兜底——
-- 无论怎么续，总量都过不了 cap，最多是买家多用几天。方向上宁可偏向买家，
-- 与分销商套餐返还额度的取舍一致。

CREATE TABLE IF NOT EXISTS user_addons (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    kind VARCHAR(20) NOT NULL,
    amount INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_addons IS '用户自助加购的并发/RPM 额度；每个 (user_id, kind) 一行，重复购买续期。';
COMMENT ON COLUMN user_addons.kind IS 'concurrency | rpm。';
COMMENT ON COLUMN user_addons.amount IS '当前持有量：并发为槽位数，RPM 为每分钟请求数。累加到 users 的对应值之上。';
COMMENT ON COLUMN user_addons.expires_at IS '到期时间；读取时即判定失效，扫描任务只负责清理。';

-- UNIQUE 而不是普通索引：一行一 (user, kind) 是这张表的不变量，靠约束守住。
-- 两个并发的首次购买会有一个撞唯一键而整笔回滚——扣款和写额度在同一个事务
-- 里，回滚不会留下"钱扣了额度没到"的残局。
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_addons_user_kind
    ON user_addons (user_id, kind);

-- 到期扫描只关心还没被清掉的行。
CREATE INDEX IF NOT EXISTS idx_user_addons_expires_at
    ON user_addons (expires_at)
    WHERE amount > 0;
