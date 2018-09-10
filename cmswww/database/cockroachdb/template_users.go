package cockroachdb

const templateCreateUserTable = `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email TEXT UNIQUE,
	username TEXT UNIQUE,
	hashed_password TEXT,
	admin BOOLEAN NOT NULL,
	register_verification_token TEXT,
	register_verification_expiry TIMESTAMP,
	update_identity_verification_token TEXT,
	update_identity_verification_expiry TIMESTAMP,
	last_login TIMESTAMP,
	failed_login_attempts INTEGER NOT NULL
);
`

const templateInsertUser = `
INSERT INTO users (%v) VALUES (%v) RETURNING id;
`

const templateUpdateUser = `
UPDATE users SET %v
WHERE %v;
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

const templateGetAllUsers = `
SELECT *
FROM users;
`

const templateDropUsersTable = `
DROP TABLE IF EXISTS users;
`
