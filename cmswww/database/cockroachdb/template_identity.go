package cockroachdb

const templateCreateIdentityTable = `
CREATE TABLE IF NOT EXISTS identity (
	user_id INTEGER REFERENCES users (id) ON DELETE CASCADE NOT NULL,
	key TEXT UNIQUE,
	activated TIMESTAMP,
	deactivated TIMESTAMP
);
`

const templateInsertIdentity = `
INSERT INTO identity (%v) VALUES (%v);
`

const templateUpdateIdentity = `
UPDATE identity SET %v
WHERE %v;
`

const templateDeleteIdentity = `
DELETE FROM identity
WHERE %v;
`

const templateGetIdentitiesByUser = `
SELECT *
FROM identity
WHERE user_id = $1;
`

const templateDropIdentityTable = `
DROP TABLE IF EXISTS identity;
`
