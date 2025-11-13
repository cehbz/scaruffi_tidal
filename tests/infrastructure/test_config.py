"""Tests for configuration management with Discogs integration"""

import unittest
import tempfile
from pathlib import Path
import yaml

from infrastructure.config import ConfigManager, AppConfiguration


class TestConfigManager(unittest.TestCase):
    """Test configuration management"""
    
    def setUp(self):
        """Create temporary config file for testing"""
        self.temp_dir = tempfile.mkdtemp()
        self.config_path = Path(self.temp_dir) / 'config.yaml'
    
    def tearDown(self):
        """Clean up temp files"""
        import shutil
        shutil.rmtree(self.temp_dir, ignore_errors=True)
    
    def test_create_default_config(self):
        """Should create default configuration"""
        config = AppConfiguration()
        
        self.assertIsNone(config.tidal_session_token)
        self.assertIsNone(config.discogs_token)
        self.assertEqual(config.discogs_rate_limit, 60)
    
    def test_load_config_with_discogs(self):
        """Should load configuration with Discogs credentials"""
        yaml_content = {
            'discogs': {
                'token': 'test_discogs_token_12345'
            },
            'tidal': {
                'session_token': 'test_tidal_token'
            }
        }
        
        with open(self.config_path, 'w') as f:
            yaml.safe_dump(yaml_content, f)
        
        manager = ConfigManager(self.config_path)
        config = manager.load()
        
        self.assertEqual(config.discogs_token, 'test_discogs_token_12345')
        self.assertEqual(config.tidal_session_token, 'test_tidal_token')
    
    def test_save_config_with_discogs(self):
        """Should save configuration with Discogs credentials"""
        manager = ConfigManager(self.config_path)
        
        config = AppConfiguration(
            discogs_token='my_discogs_token',
            tidal_session_token='my_tidal_token'
        )
        
        manager.save(config)
        
        # Verify file was created
        self.assertTrue(self.config_path.exists())
        
        # Load and verify
        with open(self.config_path, 'r') as f:
            data = yaml.safe_load(f)
        
        self.assertEqual(data['discogs']['token'], 'my_discogs_token')
        self.assertEqual(data['tidal']['session_token'], 'my_tidal_token')
    
    def test_load_nonexistent_config(self):
        """Should return default config if file doesn't exist"""
        manager = ConfigManager(self.config_path)
        config = manager.load()
        
        self.assertIsInstance(config, AppConfiguration)
        self.assertIsNone(config.discogs_token)
    
    def test_config_with_rate_limit(self):
        """Should load custom rate limit"""
        yaml_content = {
            'discogs': {
                'token': 'token123',
                'rate_limit': 30
            }
        }
        
        with open(self.config_path, 'w') as f:
            yaml.safe_dump(yaml_content, f)
        
        manager = ConfigManager(self.config_path)
        config = manager.load()
        
        self.assertEqual(config.discogs_rate_limit, 30)
    
    def test_config_requires_discogs_token(self):
        """Should validate that Discogs token is present when needed"""
        config = AppConfiguration()
        
        self.assertFalse(config.has_discogs_credentials())
        
        config = AppConfiguration(discogs_token='token123')
        
        self.assertTrue(config.has_discogs_credentials())


if __name__ == '__main__':
    unittest.main()
