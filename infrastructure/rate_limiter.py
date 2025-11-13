"""
Rate limiter for API requests.
Infrastructure layer - handles request throttling.
"""

import time
import threading
from typing import Optional


class LeakyBucketRateLimiter:
    """
    Leaky bucket rate limiter for API requests.
    
    Allows bursts up to bucket capacity while maintaining average rate.
    Thread-safe for concurrent access.
    
    The bucket "leaks" at a constant rate (requests_per_minute).
    Each request adds to the bucket. If bucket is full, caller must wait.
    """
    
    def __init__(
        self,
        requests_per_minute: int,
        bucket_capacity: Optional[int] = None
    ):
        """
        Initialize rate limiter.
        
        Args:
            requests_per_minute: Maximum sustained request rate
            bucket_capacity: Maximum burst size (defaults to requests_per_minute)
        """
        if requests_per_minute <= 0:
            raise ValueError("Requests per minute must be positive")
        
        self.requests_per_minute = requests_per_minute
        self.bucket_capacity = bucket_capacity or requests_per_minute
        
        # Rate in requests per second
        self._rate = requests_per_minute / 60.0
        
        # Current bucket level (starts full to allow immediate burst)
        self._bucket_level = 0.0
        
        # Last update time
        self._last_update = time.time()
        
        # Thread lock for concurrent access
        self._lock = threading.Lock()
        
        # Track when request started (for response time tracking)
        self._request_start_time: Optional[float] = None
    
    def acquire(self) -> None:
        """
        Acquire permission to make a request.
        
        Blocks if rate limit would be exceeded.
        Call release() after receiving the response to account for network time.
        """
        with self._lock:
            now = time.time()
            
            # Leak tokens based on time elapsed
            elapsed = now - self._last_update
            leaked = elapsed * self._rate
            self._bucket_level = max(0, self._bucket_level - leaked)
            self._last_update = now
            
            # If bucket is full, wait until it leaks enough
            if self._bucket_level >= self.bucket_capacity:
                wait_time = (self._bucket_level - self.bucket_capacity + 1) / self._rate
                time.sleep(wait_time)
                
                # Update after waiting
                now = time.time()
                elapsed = now - self._last_update
                leaked = elapsed * self._rate
                self._bucket_level = max(0, self._bucket_level - leaked)
                self._last_update = now
            
            # Add this request to bucket
            self._bucket_level += 1
            
            # Mark request start time
            self._request_start_time = now
    
    def release(self) -> None:
        """
        Mark request as complete.
        
        Uses response time (time since acquire()) to more accurately
        track request timing and account for network latency.
        """
        with self._lock:
            if self._request_start_time is None:
                return
            
            # Calculate actual request duration
            now = time.time()
            request_duration = now - self._request_start_time
            
            # Update last_update to account for request duration
            # This ensures we use response time, not request time
            self._last_update = now
            
            self._request_start_time = None
    
    def __enter__(self):
        """Context manager support."""
        self.acquire()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager support."""
        self.release()
        return False
