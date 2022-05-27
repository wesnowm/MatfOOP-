#ifndef GO_GPGME_H
#define GO_GPGME_H

#include <gpgme.h>

extern ssize_t gogpgme_readfunc(void *handle, void *buffer, size_t size);
extern ssize_t gogpgme_writefunc(void *handle, void *buffer, size_t size);
extern off_t gogpgme_seekfunc(void *handle, off_t offset, int whence);
extern gpgme_error_t gogpgme_passfunc(void *hook, char *uid_hint, char *passphrase_info, int prev_was_bad, int fd);

extern unsigned int key_revoked(gpgme_key_t k);
extern unsigned int key_expired(gpgme_key_t k);
extern unsigned int key_disabled(gpgme_key_t k);
extern unsigned int key_invalid(gpgme_key_t k);
extern unsigned int key_can_encrypt(gpgme_key_t k);
extern unsigned int key_can_sign(gpgme_key_t k);
extern unsigned int key_can_certify(gpgme_key_t k);
extern unsigned int key_secret(gpgme_key_t k);
extern unsigned int key_can_authenticate(gpgme_key_t k);
extern unsigned int key_is_qualified(gpgme_key_t k);
extern unsigned int signature_wrong_key_usage(gpgme_signature_t s);
extern unsigned int signature_pka_trust(gpgme_signature_t s);
extern unsigned int signature_chain_model(gpgme_signature_t s);
extern unsigned int subkey_revoked(gpgme_subkey_t k);
extern unsigned int subkey_expired(gpgme_subkey_t k);
extern unsigned int subkey_disabled(gpgme_subkey_t k);
extern unsigned int subkey_invalid(gpgme_subkey_t k);
extern unsigned int subkey_secret(gpgme_subkey_t k);
extern unsigned int uid_revoked(gpgme_user_id_t u);
extern unsigned int uid_invalid(gpgme_user_id_t u);

#endif
