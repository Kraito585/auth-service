CREATE TYPE auth_method AS ENUM ('password', 'password_email', 'totp', 'password_totp');

CREATE TABLE users (
    id UUID UNIQUE NOT NULL,
    login VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    hash_password VARCHAR(255),
    
    auth_preference auth_method DEFAULT 'password',
    
    totp_secret_encrypted VARCHAR(255),
    hash_totp_reset_codes JSONB,
    
    email_verified_at TIMESTAMP WITH TIME ZONE,
    totp_enabled_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
