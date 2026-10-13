-- Migration: 000001_init_schema.down.sql

DROP TRIGGER IF EXISTS trg_prevent_entry_mutation ON entries;
DROP FUNCTION IF EXISTS prevent_entry_mutation();
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
