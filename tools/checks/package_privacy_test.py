"""Regression checks for certificate contacts and compressed PE debug scanning."""
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

def pe_debug(payload,tail=b''):
    blob=b'ZLIB'+len(payload).to_bytes(8,'big')+zlib.compress(payload)+tail
    strings=struct.pack('<I',16)+b'.zdebug_info\0'
    raw=bytearray(512+len(blob)+len(strings))
    raw[:2]=b'MZ';struct.pack_into('<I',raw,60,128)
    raw[128:132]=b'PE\0\0'
    struct.pack_into('<H',raw,134,1)
    struct.pack_into('<I',raw,140,512+len(blob))
    struct.pack_into('<H',raw,148,240)
    raw[392:400]=b'/4\0\0\0\0\0\0'
    struct.pack_into('<II',raw,408,len(blob),512)
    raw[512:512+len(blob)]=blob
    raw[512+len(blob):]=strings
    return bytes(raw)

class PrivacyTests(unittest.TestCase):
    def test_compressed_debug_is_scanned_as_decompressed_text(self):
        payload=b'Go debug symbols: trimmed/module/main.go'
        raw=pe_debug(payload)
        original=bytes(raw)
        scanned=privacy_scan_bytes(raw)
        self.assertTrue(scanned.endswith(payload))
        self.assertNotEqual(scanned,raw)
        privacy('helper.exe',raw)
        self.assertEqual(raw,original)

    def test_private_values_inside_compressed_debug_are_rejected(self):
        for value in (PRIVATE,PERSONAL,PRIVATE.decode().encode('utf-16-le')):
            with self.subTest(value=value):
                with self.assertRaises(AssertionError):privacy('helper.exe',pe_debug(value))

    def test_bytes_after_compressed_debug_stream_stay_scanned(self):
        with self.assertRaises(AssertionError):privacy('helper.exe',pe_debug(b'public symbols',PRIVATE))
        with self.assertRaises(AssertionError):privacy('helper.exe',b'MZ'+PRIVATE)

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
