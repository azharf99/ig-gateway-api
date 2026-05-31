#!/bin/sh

# Fix ownership and permissions of the uploads directory dynamically at runtime
chown -R appuser:appgroup /app/uploads
chmod 770 /app/uploads

# Drop privileges and execute the application as appuser
exec su-exec appuser ./api
