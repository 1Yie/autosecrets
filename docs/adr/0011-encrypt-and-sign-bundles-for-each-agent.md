# Encrypt and sign Bundles for each Agent

Core will encrypt every downloadable Bundle payload to the assigned Agent's independent encryption public key and sign the envelope before delivery over mTLS. The additional message layer prevents proxies, caches, or a payload sent to the wrong node from exposing Secret values, while the signature lets the Agent reject forged Desired State.
