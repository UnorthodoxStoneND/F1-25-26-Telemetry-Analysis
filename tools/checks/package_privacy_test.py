"""Regression checks for the precise public certificate contact exception."""
import struct
import unittest
import zlib

from package_check import privacy, privacy_scan_bytes

PNG=b'\x89PNG\r\n\x1a\n'
CONTACT='@'.join(('ca','trufo.ai')).encode()
DN=bytes.fromhex('301806092a864886f70d010901160b')
PRIVATE='@'.join(('private-person','example.org')).encode()
PERSONAL=('C:'+chr(92)+'Users'+chr(92)+'private-person'+chr(92)+'file').encode()

def chunk(kind,payload):
    return struct.pack('>I',len(payload))+kind+payload+struct.pack('>I',zlib.crc32(kind+payload))

class PrivacyTests(unittest.TestCase):
    def test_public_ca_dn_in_provenance_is_allowed_without_mutation(self):
        raw=PNG+chunk(b'caBX',DN+CONTACT)
        original=bytes(raw)
        privacy('icon.png',raw)
        self.assertEqual(raw,original)
        self.assertNotEqual(privacy_scan_bytes(raw),raw)

    def test_other_locations_and_private_content_are_rejected(self):
        for raw in [CONTACT,DN+CONTACT,PNG+chunk(b'tEXt',DN+CONTACT),
                    PNG+chunk(b'caBX',CONTACT),PNG+chunk(b'caBX',DN+PRIVATE),
                    PNG+chunk(b'caBX',DN+CONTACT+PRIVATE),
                    PNG+chunk(b'caBX',DN+CONTACT+PERSONAL),
                    PERSONAL,PRIVATE]:
            with self.subTest(raw=raw):
                with self.assertRaises(AssertionError):privacy('fixture',raw)

    def test_truncated_chunk_does_not_hide_contacts(self):
        raw=PNG+struct.pack('>I',9999)+b'caBX'+DN+CONTACT
        with self.assertRaises(AssertionError):privacy('fixture',raw)

if __name__=='__main__':unittest.main()
