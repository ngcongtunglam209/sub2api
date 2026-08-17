-- 分销商域名的品牌覆盖：api.brand.com 要显示分销商自己的名字、logo 和副标题。
--
-- 列挂在 reseller_domains 上而不是新开一张表：品牌是"这个域名长什么样"，
-- 与 Host 一一对应，独立成表只会多一次 join 和一份可能对不上的行。
--
-- 三列全部可空，且**逐字段**回退到全站设置。NULL 与空串同义，都表示"用全站
-- 的那个值"，而不是"显示空白"——只改名字不换 logo 是最常见的配置，逐字段回退
-- 让它不必把全站 logo 抄一遍。管理端把空串当作"清除覆盖"，靠的正是这条等价。
--
-- 没有默认品牌行、没有回填：迁移跑完，所有既有域名的表现与迁移前完全一致。
ALTER TABLE reseller_domains
    ADD COLUMN IF NOT EXISTS site_name VARCHAR(100),
    ADD COLUMN IF NOT EXISTS site_logo TEXT,
    ADD COLUMN IF NOT EXISTS site_subtitle VARCHAR(200);

COMMENT ON COLUMN reseller_domains.site_name IS '该域名下的站点名覆盖；NULL/空串表示沿用全站 site_name。';
COMMENT ON COLUMN reseller_domains.site_logo IS '该域名下的 logo/favicon 覆盖；NULL/空串表示沿用全站 site_logo。';
COMMENT ON COLUMN reseller_domains.site_subtitle IS '该域名下的副标题覆盖；NULL/空串表示沿用全站 site_subtitle。';
