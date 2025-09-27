# cv-job-matching

## Secrets and SOPS

This repository uses SOPS (with age) to encrypt sensitive files so they can be committed safely.

- Encrypted artifacts:
  - `project.md.sops` (encrypted project brief)
  - Optional: `.env.sops.yaml` (encrypted environment configuration)
- Do NOT commit plaintext files such as `project.md` or `.env`. Both are listed in `.gitignore`.

### Local prerequisites
- Install `sops` and `age`.
- Ensure your age private key exists at `~/.config/sops/age/keys.txt`.
- Your public recipient is printed by:
  ```bash
  age-keygen -y ~/.config/sops/age/keys.txt
  ```

### Decrypt
- Project brief:
  ```bash
  sops -d --input-type binary --output-type binary project.md.sops > project.md
  ```
- Encrypted env:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```

### Edit and re-encrypt
For `project.md.sops` (binary), decrypt to plaintext, edit, then re-encrypt:
```bash
# decrypt to plaintext, edit it
sops -d --input-type binary --output-type binary project.md.sops > project.md

# re-encrypt to .sops using your age recipient
sops --encrypt --age "$(age-keygen -y ~/.config/sops/age/keys.txt)" \
  --input-type binary --output-type binary project.md > project.md.sops
```

For `.env.sops.yaml` (YAML), you can edit in place and SOPS will re-encrypt on save:
```bash
sops .env.sops.yaml
```

### CI/CD
- Store the age private key in a secret (e.g., `SOPS_AGE_KEY`).
- In CI, write the key to `~/.config/sops/age/keys.txt`, then decrypt:
  ```bash
  sops -d .env.sops.yaml > .env
  chmod 600 .env
  ```
- See the CI rules for a full example.
