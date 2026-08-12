# Transport Secret Values as bounded base64

The management API will carry Secret Value bytes as explicit base64 in JSON and reject values larger than 1 MiB after decoding, with the request-body limit accounting for base64 expansion. Text input is UTF-8 encoded before transport, selected files retain their exact bytes, local filenames remain browser-only suggestions, and no response returns the value; this keeps one typed JSON contract while preserving the domain promise that Secret Versions contain opaque bytes rather than only valid text.
