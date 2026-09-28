-- +goose Up
ALTER TABLE runtime_deployment ADD COLUMN e2b_api_url text NOT NULL DEFAULT '';
ALTER TABLE runtime_deployment ADD COLUMN e2b_domain text NOT NULL DEFAULT '';
ALTER TABLE runtime_deployment ADD CONSTRAINT runtime_deployment_e2b_endpoint_check CHECK (
    (provider_kind = 'e2b' AND ((e2b_api_url = '' AND e2b_domain = '') OR
                                 (e2b_api_url <> '' AND e2b_domain <> ''))) OR
    (provider_kind <> 'e2b' AND e2b_api_url = '' AND e2b_domain = '')
);

-- +goose Down
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM runtime_deployment WHERE e2b_api_url <> '' OR e2b_domain <> '') THEN
        RAISE EXCEPTION 'cannot remove E2B endpoint columns while a custom endpoint is configured';
    END IF;
END $$;
ALTER TABLE runtime_deployment DROP CONSTRAINT runtime_deployment_e2b_endpoint_check;
ALTER TABLE runtime_deployment DROP COLUMN e2b_domain;
ALTER TABLE runtime_deployment DROP COLUMN e2b_api_url;
