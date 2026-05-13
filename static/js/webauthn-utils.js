(function () {
    function base64URLToBuffer(value) {
        var base64 = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
        var padded = base64.padEnd(base64.length + (4 - base64.length % 4) % 4, '=');
        var binary = atob(padded);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    function bufferToBase64URL(buffer) {
        var bytes = new Uint8Array(buffer);
        var binary = '';
        for (var i = 0; i < bytes.byteLength; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
    }

    function prepareAssertionOptions(options) {
        var publicKey = options.publicKey || options;
        publicKey.challenge = base64URLToBuffer(publicKey.challenge);
        if (publicKey.allowCredentials) {
            publicKey.allowCredentials = publicKey.allowCredentials.map(function (credential) {
                return Object.assign({}, credential, { id: base64URLToBuffer(credential.id) });
            });
        }
        return publicKey;
    }

    function serializeAssertion(credential) {
        return {
            id: credential.id,
            rawId: bufferToBase64URL(credential.rawId),
            type: credential.type,
            response: {
                authenticatorData: bufferToBase64URL(credential.response.authenticatorData),
                clientDataJSON: bufferToBase64URL(credential.response.clientDataJSON),
                signature: bufferToBase64URL(credential.response.signature),
                userHandle: credential.response.userHandle ? bufferToBase64URL(credential.response.userHandle) : null
            },
            clientExtensionResults: credential.getClientExtensionResults()
        };
    }

    async function fetchJSON(url, options) {
        var response = await fetch(url, Object.assign({
            method: 'POST',
            credentials: 'include',
            headers: { Accept: 'application/json' }
        }, options || {}));
        var data = {};
        try {
            data = await response.json();
        } catch (_) {}
        if (!response.ok || !data.success) {
            throw new Error(data.error || 'Passkey login failed.');
        }
        return data;
    }

    async function passkeyLogin(config) {
        if (!window.PublicKeyCredential || !navigator.credentials) {
            throw new Error('This browser does not support passkeys.');
        }

        var beginData = await fetchJSON(config.beginUrl);
        var credential = await navigator.credentials.get({
            publicKey: prepareAssertionOptions(beginData.options)
        });
        if (!credential) {
            throw new Error('Passkey login was cancelled.');
        }

        return fetchJSON(config.finishUrl, {
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(serializeAssertion(credential))
        });
    }

    window.GoatFlowWebAuthn = {
        base64URLToBuffer: base64URLToBuffer,
        bufferToBase64URL: bufferToBase64URL,
        prepareAssertionOptions: prepareAssertionOptions,
        serializeAssertion: serializeAssertion,
        passkeyLogin: passkeyLogin
    };
})();
