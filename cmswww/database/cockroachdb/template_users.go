package cockroachdb

const templateCreateUserTable = `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email TEXT UNIQUE,
	username TEXT UNIQUE NOT NULL,
	hashed_password TEXT NOT NULL,
	admin BOOLEAN NOT NULL,
	register_verification_token TEXT,
	register_verification_expiry TIMESTAMP,
	update_identity_verification_token TEXT,
	update_identity_verification_expiry TIMESTAMP,
	last_login TIMESTAMP,
	failed_login_attempts INT NOT NULL
);
`

const templateInsertUser = `
INSERT INTO users (
	email,
	username,
	hashed_password,
	admin,
	register_verification_token,
	register_verification_expiry,
	update_identity_verification_token,
	update_identity_verification_expiry,
	failed_login_attempts
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
`

const templateGetUserByID = `
SELECT *
FROM users
WHERE id = $1;
`

const templateGetUserByEmail = `
SELECT *
FROM users
WHERE LOWER(email) = LOWER($1);
`

const templateGetUserByUsername = `
SELECT *
FROM users
WHERE LOWER(username) = LOWER($1);
`
