import math  # Import math module for advanced operations (though not strictly needed here)

def is_prime(n):
    """Checks if a number is prime."""
    if n <= 1:  # Numbers less than or equal to 1 are not prime
        return False
    if n <= 3:  # 2 and 3 are prime
        return True
    if n % 2 == 0 or n % 3 == 0:  # Multiples of 2 and 3 are not prime
        return False
    i = 5  # Start checking divisors from 5
    while i * i <= n:  # Check divisors up to square root of n
        if n % i == 0 or n % (i + 2) == 0:  # Check if n is divisible by i or i+2
            return False
        i += 6  # Increment divisor by 6 to skip even numbers and multiples of 3
    return True  # If no divisors found, number is prime

if __name__ == "__main__":
    import sys  # Import sys to access command line arguments
    if len(sys.argv) > 1:  # Check if an argument was passed
        try:
            num = int(sys.argv[1])  # Convert first argument to integer
            print(f"{num} is {'prime' if is_prime(num) else 'not prime'}")  # Print prime status
        except ValueError:  # Handle cases where argument is not an integer
            print("Please provide a valid integer.")
    else:
        print("Usage: python tests/prime_check.py <number>")  # Prompt user for input

