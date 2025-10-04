CREATE INDEX IF NOT EXISTS user_account_email_idx ON user_account (LOWER(email));
CREATE INDEX IF NOT EXISTS user_account_username_idx ON user_account (LOWER(username));
CREATE INDEX IF NOT EXISTS travel_destination_category_idx ON travel_destination (category);
CREATE INDEX IF NOT EXISTS travel_destination_city_idx ON travel_destination (city);
