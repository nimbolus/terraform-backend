terraform {
  backend "http" {
    address        = "https://dummy-backend.example.com/state"
    lock_address   = "https://dummy-backend.example.com/state"
    unlock_address = "https://dummy-backend.example.com/state"
    username       = "my_user"
  }
}
