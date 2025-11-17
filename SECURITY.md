# Security Policy

## Overview

This document outlines security measures implemented in the Lemmy Image Scraper and provides guidance for secure deployment.

## Security Features

### 1. Path Traversal Protection

**Location:** `internal/web/server.go`

The web server implements multi-layered path traversal protection:
- Path normalization using `filepath.Clean()`
- Rejection of absolute paths and `..` sequences
- Symlink resolution with `filepath.EvalSymlinks()`
- Validation that resolved paths stay within the base directory
- Logging of blocked traversal attempts

### 2. HTTP Security Headers

All HTTP responses include comprehensive security headers:
- **X-Content-Type-Options**: `nosniff` - Prevents MIME sniffing attacks
- **X-Frame-Options**: `DENY` - Prevents clickjacking
- **X-XSS-Protection**: `1; mode=block` - Legacy XSS protection
- **Referrer-Policy**: `strict-origin-when-cross-origin` - Controls referrer information
- **Content-Security-Policy**: Restrictive CSP limiting script sources
- **Permissions-Policy**: Disables unnecessary browser features

### 3. File Size Limits

**Location:** `internal/downloader/downloader.go`

Downloaded files are limited to **500 MB** to prevent:
- Memory exhaustion attacks
- Disk space exhaustion
- Resource abuse

Size limits are enforced at two levels:
1. Content-Length header validation (early rejection)
2. Stream limiting with `io.LimitReader` (prevents bypass)

### 4. SSRF Prevention

**Location:** `internal/downloader/downloader.go:validateURL()`

URL validation prevents Server-Side Request Forgery attacks by:
- Allowing only HTTP and HTTPS schemes
- Blocking localhost addresses (`127.0.0.1`, `::1`, etc.)
- Blocking private IP ranges:
  - `10.0.0.0/8`
  - `172.16.0.0/12`
  - `192.168.0.0/16`
  - `169.254.0.0/16` (link-local)
  - IPv6 private ranges (`fc00::`, `fd00::`)

### 5. Secure File Permissions

Downloaded files and directories use restrictive permissions:
- **Directories**: `0700` (owner read/write/execute only)
- **Files**: `0600` (owner read/write only)

This prevents unauthorized access in multi-user environments.

### 6. SQL Injection Protection

**Location:** `internal/database/database.go`

All database queries use:
- Prepared statements with parameterized queries
- Whitelist validation for dynamic fields (sort columns)
- No direct string concatenation in SQL

### 7. Input Sanitization

- **Path names**: Invalid characters are replaced with underscores
- **Filenames**: Sanitized to prevent filesystem issues
- **Community names**: Sanitized before use in paths

## Known Limitations & Recommendations

### 1. Credential Storage

⚠️ **IMPORTANT**: Lemmy credentials are currently stored in **plaintext** in `config.yaml`.

**Recommendations:**
- Set strict file permissions: `chmod 600 config.yaml`
- Never commit `config.yaml` to version control (it's in `.gitignore`)
- Consider using environment variables for credentials:
  ```bash
  export LEMMY_USERNAME="your_username"
  export LEMMY_PASSWORD="your_password"
  ```
- For production deployments, use a secret management system (e.g., HashiCorp Vault, AWS Secrets Manager)

### 2. JWT Token Handling

JWT authentication tokens are stored in memory during runtime. While this is standard practice:
- Tokens are not persisted to disk
- Tokens are cleared when the process exits
- For enhanced security, consider implementing token rotation

### 3. HTTPS/TLS

The scraper enforces HTTPS when connecting to Lemmy instances. However:
- TLS certificate validation uses Go's default settings
- For self-hosted Lemmy instances with self-signed certificates, you may need to add the CA to your system's trust store
- Do not disable certificate validation in production

### 4. Web Server Security

The included web server is designed for **local use only**. For production deployments:
- Place behind a reverse proxy (nginx, Apache, Caddy)
- Enable HTTPS with valid certificates
- Implement authentication (the web server has no built-in auth)
- Use firewall rules to restrict access
- Set `web_server.host` to `localhost` unless you need external access

## Deployment Security Checklist

### Minimal Setup
- [ ] Set `config.yaml` permissions to `0600`
- [ ] Review and set appropriate `storage.base_directory`
- [ ] Ensure database path is in a secure location
- [ ] If enabling web server, bind to `localhost` only

### Recommended Setup
- [ ] Use environment variables for credentials
- [ ] Run as a dedicated non-root user
- [ ] Use systemd or similar to manage the process
- [ ] Configure log rotation
- [ ] Set up firewall rules
- [ ] Regular security updates for dependencies

### Production Setup
- [ ] Use a secrets management system
- [ ] Deploy web server behind reverse proxy with HTTPS
- [ ] Implement authentication on the web interface
- [ ] Set up monitoring and alerting
- [ ] Regular security audits
- [ ] Backup database regularly
- [ ] Use container security scanning if using Docker

## Reporting Security Issues

If you discover a security vulnerability, please email the maintainers directly rather than opening a public issue. Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Security Update Policy

Security patches will be prioritized and released as soon as possible. When updating:
1. Review the changelog for security-related changes
2. Test in a non-production environment first
3. Backup your database before upgrading
4. Check for configuration changes

## Security Audit History

| Date       | Type              | Findings | Status   |
|------------|-------------------|----------|----------|
| 2025-01-17 | Automated Review  | 19       | Resolved |

## Additional Resources

- [OWASP Top Ten](https://owasp.org/www-project-top-ten/)
- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [Lemmy API Documentation](https://join-lemmy.org/api/)

## License

This security policy is part of the project and follows the same license terms.
