# Velarix Session State

**Current Task**: Step 4.1: Real Identity Provider
**Status**: In Progress (Defining Schema)

## Last Completed Task
- **Step 3.4: Global Re-Extraction**
    - Implemented revalidation endpoint with impact summary.
    - Verified logic replay from journal.

## Next Steps (Step 4.1: Real Identity Provider)
1. **[ ] User Store**: Define `User` and `APIKey` structs in BadgerDB.
2. **[ ] Argon2 Hashing**: Implement secure password storage.
3. **[ ] Auth Routes**: Register, Login, Reset-Request (Console log), and Reset-Confirm.
4. **[ ] Revocable Keys**: Support multiple keys per user with unified 401 error responses.

## Active Context & Constraints
- **Security Parity**: Missing vs. Revoked keys must return identical 401s.
- **Console Reset**: Reset tokens logged to stdout for now.
- **Conduct**: Numerical headers, checkboxes, and technical sub-bullets.
