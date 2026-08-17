-- 兑换码归属：记录是谁生成的。
--
-- 分销商用自己的余额生成兑换码再转售，因此必须能把码算到人头上：对账、
-- 停用某个分销商后追查其未售出的码、以及分销商面板只列自己的码，都靠这一列。
--
-- 可空：此列之前生成的码、以及管理员在后台直接生成的码都没有归属，
-- NULL 表示「平台自己发的」，不是缺数据。

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS created_by BIGINT;

COMMENT ON COLUMN redeem_codes.created_by IS '生成该码的用户；NULL = 平台/管理员生成。';

-- 分销商面板按 created_by 列自己的码，未使用的码要能快速筛出来。
CREATE INDEX IF NOT EXISTS idx_redeem_codes_created_by
    ON redeem_codes (created_by, status)
    WHERE created_by IS NOT NULL;
