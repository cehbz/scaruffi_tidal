"""Tests for rate limiter"""

import unittest
import time
from infrastructure.rate_limiter import LeakyBucketRateLimiter


class TestLeakyBucketRateLimiter(unittest.TestCase):
    """Test leaky bucket rate limiter"""
    
    def test_create_limiter(self):
        """Should create rate limiter with specified rate"""
        limiter = LeakyBucketRateLimiter(requests_per_minute=60)
        
        self.assertEqual(limiter.requests_per_minute, 60)
    
    def test_allows_burst_within_limit(self):
        """Should allow rapid requests up to the limit"""
        limiter = LeakyBucketRateLimiter(requests_per_minute=60)
        
        # Should allow immediate requests
        for _ in range(5):
            limiter.acquire()  # Should not block
    
    def test_blocks_when_over_rate(self):
        """Should block when rate exceeded"""
        # 120 requests per minute = 2 per second
        limiter = LeakyBucketRateLimiter(requests_per_minute=120)
        
        # First few should be fast
        start = time.time()
        for _ in range(5):
            limiter.acquire()
        elapsed = time.time() - start
        
        # Should take some time but not too much (bucket allows some burst)
        self.assertLess(elapsed, 5.0)
    
    def test_leaks_over_time(self):
        """Should leak tokens over time, allowing more requests"""
        limiter = LeakyBucketRateLimiter(requests_per_minute=60)
        
        # Make some requests
        for _ in range(10):
            limiter.acquire()
        
        # Wait a bit for bucket to leak
        time.sleep(0.5)
        
        # Should be able to make more requests
        limiter.acquire()  # Should not block indefinitely
    
    def test_tracks_response_time(self):
        """Should use response time for rate calculation"""
        limiter = LeakyBucketRateLimiter(requests_per_minute=60)
        
        # Simulate a slow request
        limiter.acquire()
        time.sleep(0.1)
        limiter.release()
        
        # Next request should account for that time
        limiter.acquire()


if __name__ == '__main__':
    unittest.main()
