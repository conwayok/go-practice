CREATE ROLE bank_user LOGIN ENCRYPTED PASSWORD 'bank_user_pass';
CREATE DATABASE bank;
\c bank;
CREATE SCHEMA bank;
ALTER SCHEMA bank OWNER TO bank_user;
ALTER ROLE bank_user SET SEARCH_PATH = 'bank';
