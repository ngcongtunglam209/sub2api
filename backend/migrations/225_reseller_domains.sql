-- 分销商自定义域名：分销商把自己的域名 CNAME 到本站，其客户只会看到
-- 分销商的域名，看不到上游是谁。
--
-- 这张表同时承担两个职责，缺一不可：
--   1. Caddy 的 on_demand_tls ask 端点据此决定是否为陌生 host 申请证书。
--      没有白名单，任何人把域名解析过来都能逼我们签证书，既是被滥用的入口，
--      也会把 Let's Encrypt 的签发额度耗光。
--   2. 请求准入。ask 只管签证书，不管流量：把分销商停用后，已签发的证书
--      仍有效近 90 天，若不在应用层按 Host 拦截，"停用"三个月内形同虚设。

CREATE TABLE IF NOT EXISTS reseller_domains (
    id BIGSERIAL PRIMARY KEY,
    -- DNS 名称上限 253 字符。统一以小写存储，查询前也小写化，
    -- 因为 Host 头的大小写由客户端决定，不能指望它规范。
    domain VARCHAR(253) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE reseller_domains IS '分销商自定义域名白名单；同时用于签发证书与请求准入。';
COMMENT ON COLUMN reseller_domains.domain IS '小写存储的完整域名，如 api.brand.com。';
COMMENT ON COLUMN reseller_domains.user_id IS '归属的分销商用户；用户被删除后由后台清理，不做级联。';
COMMENT ON COLUMN reseller_domains.status IS 'active 才签发证书并放行流量；其余值立即失效。';

-- 每次与陌生 host 握手都会查一次，且请求准入也走同一条路径。
-- domain 上的唯一约束已提供索引，这里再补一条带 status 的覆盖索引，
-- 让常态查询（status='active'）不必回表。
CREATE INDEX IF NOT EXISTS idx_reseller_domains_active
    ON reseller_domains (domain)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_reseller_domains_user_id
    ON reseller_domains (user_id);
