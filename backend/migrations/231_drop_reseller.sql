-- 移除分销商功能：套餐、自定义域名、以及挂在域名上的品牌覆盖。
--
-- 225/226/229 保留在历史里不动——它们已经在生产库跑过，改动会破坏 checksum
-- 校验。这条迁移只做正向拆除，让已部署的库与新代码对齐。
--
-- 顺序：先摘 users 上的索引与列，再删两张表。users 的两个索引带
-- WHERE reseller_plan_id IS NOT NULL，必须在删列之前先删掉，否则
-- DROP COLUMN 会因索引依赖而级联删除——结果一样，但显式写出来更清楚。
--
-- reseller_domains 上的 site_name / site_logo / site_subtitle（229 加的品牌
-- 覆盖列）随表一起消失，不必单独 DROP COLUMN。全站品牌回落到 settings 里的
-- site_name / site_logo / site_subtitle，与未启用分销商时的表现一致。

DROP INDEX IF EXISTS idx_users_reseller_plan_expires_at;
DROP INDEX IF EXISTS idx_users_reseller_plan_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS reseller_plan_id,
    DROP COLUMN IF EXISTS reseller_plan_expires_at;

-- 两张表各自的索引（idx_reseller_domains_active、idx_reseller_domains_user_id、
-- idx_reseller_plans_enabled_level）随表删除，无需单列。
DROP TABLE IF EXISTS reseller_domains;
DROP TABLE IF EXISTS reseller_plans;
