-- +goose Up
-- +goose StatementBegin
UPDATE custom_command_aliases
SET alias = '!' || alias
WHERE alias <> '' AND alias NOT LIKE '!%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE custom_command_aliases
SET alias = substring(alias FROM 2)
WHERE alias LIKE '!%';
-- +goose StatementEnd
