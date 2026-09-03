# Troubleshooting

Common issues and their solutions.

## Startup Issues

### Server won't start

**Symptoms:** Server exits immediately or shows error on startup.

**Check:**

1. **Port already in use:**
   ```bash
   # Check if port is in use
   lsof -i :64893

   # Use a different port
   vidveil --port 64894
   ```

2. **Permission denied:**
   ```bash
   # Ports below 1024 require root
   sudo vidveil --port 443

   # Or use a higher port
   vidveil --port 8443
   ```

3. **Missing configuration:**
   ```bash
   # Check config directory
   vidveil --config /path/to/config-dir

   # First startup creates server.yml automatically inside that directory
   vidveil --config /path/to/config-dir --port 64893
   ```

### Database errors

**Symptoms:** `database is locked` or `no such table`

**Solutions:**

1. **Database locked:**
   ```bash
   # Another process may be using the database
   lsof ~/.local/share/apimgr/vidveil/db/server.db

   # Stop the stale process by PID
   kill <PID>
   ```

2. **Corrupted database:**
   ```bash
   # Backup and recreate
   mv ~/.local/share/apimgr/vidveil/db/server.db server.db.bak
   vidveil  # Will create new database
   ```

3. **Run migrations:**
   ```bash
   # Migrations run automatically on startup
   systemctl restart vidveil
   ```

---

## Admin Panel Issues

### Can't log in

**Symptoms:** Login fails with "Invalid credentials"

**Solutions:**

1. **Reset admin password:**
   ```bash
   vidveil --maintenance setup
   # Follow prompts to create new admin
   ```

2. **Check setup token:**
   - On first run, check console for setup token
   - Navigate to `https://your-domain.example/admin` and enter the token

3. **Clear browser cache:**
   - Clear cookies for the site
   - Try incognito/private window

### Lost setup token

**Symptoms:** Can't complete initial setup

**Solution:**

```bash
# Regenerate setup token
vidveil --maintenance setup

# Token will be displayed in console
```

### 2FA locked out

**Symptoms:** Can't access account after losing 2FA device

**Solutions:**

1. **Use recovery key:**
   - Enter one of your 8 backup codes
   - Each code works once

2. **Reset via command line:**
   ```bash
   # If recovery keys are unavailable, reset admin credentials
   vidveil --maintenance setup
   ```

---

## Search Issues

### No search results

**Symptoms:** Searches return empty results

**Check:**

1. **Engine status:**
   - Go to Admin → Engines
   - Check if engines are enabled
   - Test individual engine connectivity

2. **Network issues:**
   ```bash
   # Test connectivity to engines
   curl -q -LSsfI https://www.pornhub.com
   ```

3. **Rate limiting:**
   - Check if you're being rate limited
   - Wait a few minutes and try again

### Slow searches

**Symptoms:** Searches take too long

**Solutions:**

1. **Enable caching:**
   - Go to Admin → Server → Settings
   - Enable search result caching
   - Set appropriate cache TTL

2. **Reduce engines:**
   - Disable slow or unreliable engines
   - Prioritize faster engines

3. **Check server resources:**
   ```bash
   # Monitor CPU/memory
   htop

   # Check disk I/O
   iotop
   ```

### Engine errors

**Symptoms:** Specific engines failing

**Check:**

1. **Engine website status:**
   - Site may be down or blocking
   - Check from different network

2. **Update engine:**
   - Admin → Updates
   - Check for engine updates

3. **View logs:**
   ```bash
   tail -f ~/.local/log/apimgr/vidveil/error.log
   ```

---

## SSL/TLS Issues

### Certificate errors

**Symptoms:** Browser shows certificate warnings

**Solutions:**

1. **Check certificate validity:**
   ```bash
   openssl s_client -connect your-domain.example:443 \
     -servername your-domain.example
   ```

2. **Renew certificate:**
   - Admin → Server → SSL
   - Click "Renew Now"

3. **Check DNS:**
   ```bash
   dig your-domain.example
   # Ensure A/AAAA records point to server
   ```

### Let's Encrypt fails

**Symptoms:** Certificate renewal fails

**Check:**

1. **Port 80 accessible:**
   ```bash
   # HTTP-01 challenge requires port 80
   curl -q -LSsf http://your-domain.example/.well-known/acme-challenge/test
   ```

2. **DNS propagation:**
   ```bash
   # For DNS-01 challenge
   dig _acme-challenge.your-domain.example TXT
   ```

3. **Rate limits:**
   - Let's Encrypt has rate limits
   - Wait and retry in 1 hour

---

## Performance Issues

### High CPU usage

**Symptoms:** Server using excessive CPU

**Solutions:**

1. **Check for runaway searches:**
   ```bash
   # View active goroutines
   curl -q -LSsf http://127.0.0.1:64893/debug/pprof/goroutine?debug=2
   ```

2. **Enable rate limiting:**
   - Admin → Security → Rate Limiting
   - Set appropriate limits

3. **Reduce concurrent engines:**
   - Lower max concurrent searches
   - Disable unnecessary engines

### High memory usage

**Symptoms:** Memory grows over time

**Solutions:**

1. **Clear cache:**
   - Open Admin → Server → Maintenance
   - Use **Clear Search Cache** or **Clear All Caches**

2. **Reduce cache size:**
   - Admin → Server → Settings
   - Lower cache max entries

3. **Restart server:**
   ```bash
   systemctl restart vidveil
   ```

### Slow page loads

**Symptoms:** Admin panel or pages load slowly

**Check:**

1. **Database size:**
   ```bash
   ls -lh ~/.local/share/apimgr/vidveil/db/server.db
   ```
   - If large, open Admin → Server → Database or Admin → Server → Maintenance
   - Run **Vacuum Database**

2. **Log rotation:**
   - Review `server.logs.*` rotation settings in `server.yml`
   - Restart the service if you need log files reopened immediately

---

## Docker Issues

### Container won't start

**Symptoms:** Container exits immediately

**Check logs:**

```bash
docker logs vidveil
```

**Common issues:**

1. **Volume permissions:**
   ```bash
   # Fix ownership
   sudo chown -R 1000:1000 ./rootfs
   ```

2. **Port conflicts:**
   ```bash
   docker run -p 64581:80 ghcr.io/apimgr/vidveil:latest
   ```

### Data persistence

**Symptoms:** Data lost after restart

**Solution:**

```bash
# Use volumes for persistence
docker run -d \
  --name vidveil \
  -p 64580:80 \
  -v ./rootfs/config:/config:z \
  -v ./rootfs/data:/data:z \
  ghcr.io/apimgr/vidveil:latest
```

---

## Network Issues

### Behind reverse proxy

**Symptoms:** Wrong IP addresses, SSL issues

**Configuration:**

1. **Set trusted proxies:**
   ```yaml
   # server.yml
   server:
     trusted_proxies:
       - "10.0.0.0/8"
       - "172.16.0.0/12"
       - "192.168.0.0/16"
   ```

2. **Forward headers:**
    ```nginx
    # nginx.conf
    location / {
        proxy_pass http://127.0.0.1:64893;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
       proxy_set_header Host $host;
   }
   ```

### CORS errors

**Symptoms:** API calls blocked by browser

**Solution:**

```yaml
# server.yml
server:
  cors:
    enabled: true
    origins:
      - "https://your-frontend.com"
```

---

## Logging & Debugging

### Enable debug logging

```bash
vidveil --debug
```

Or in config:

```yaml
server:
  logs:
    level: debug
    debug:
      enabled: true
```

### View logs

```bash
# All logs
tail -f ~/.local/log/apimgr/vidveil/server.log

# Errors only
tail -f ~/.local/log/apimgr/vidveil/error.log

# Access log
tail -f ~/.local/log/apimgr/vidveil/access.log

# Audit log
tail -f ~/.local/log/apimgr/vidveil/audit.log
```

### Debug endpoints

Available only when the server is started with `--debug` (or `DEBUG=true` in containerized runs):

```bash
# CPU profile
curl -q -LSsf http://127.0.0.1:64893/debug/pprof/profile > cpu.prof
go tool pprof cpu.prof

# Memory profile
curl -q -LSsf http://127.0.0.1:64893/debug/pprof/heap > heap.prof

# Goroutines
curl -q -LSsf http://127.0.0.1:64893/debug/pprof/goroutine?debug=2
```

---

## Getting Help

If you can't resolve an issue:

1. **Check existing issues:**
   - [GitHub Issues](https://github.com/apimgr/vidveil/issues)

2. **Gather information:**
   ```bash
   # Version
   vidveil --version

   # System info
   vidveil --status

   # Relevant logs
   tail -100 ~/.local/log/apimgr/vidveil/error.log
   ```

3. **Open new issue:**
   - Include version, OS, and steps to reproduce
   - Attach relevant logs (remove sensitive info)
