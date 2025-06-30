CREATE TABLE sms_messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    message_sid VARCHAR(50) NOT NULL,
    from_number VARCHAR(15) NOT NULL,
    body TEXT NOT NULL,
    received_at DATETIME NOT NULL
);