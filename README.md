# GoDB - Solana Pump.fun API Integration

**Note: This documentation may be slightly outdated as the codebase continues to evolve.**

## Overview

GoDB is a Go-based backend service that provides a REST API for interacting with the Pump.fun platform. This service allows authenticated users to post comments, batch comments, and like messages on Pump.fun.

## Architecture

The application is structured into three main packages:

- **handlers**: HTTP request handlers that process API endpoints
- **database**: Database interaction layer for user authentication and operation tracking
- **pump**: Client implementation for interfacing with the Pump.fun API

## Features

- User authentication via Solana wallet signatures
- Post individual comments on Pump.fun
- Batch comment posting with random delays
- Like messages on Pump.fun
- Asynchronous operation tracking for batch operations

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sol-route/comment` | POST | Post a single comment |
| `/sol-route/batch-comments` | POST | Post multiple comments with configurable delays |
| `/sol-route/like` | POST | Like a specific message |
| `/sol-route/operation-status` | GET | Check status of a batch operation |

## Setup

### Prerequisites

- Go 1.23.0 or higher
- MySQL database
- Solana wallet for authentication

### Database Configuration

The application requires a MySQL database with an `auth_bot` schema containing:
- `auth` table (for storing user tokens)
- `comment_operations` table (for tracking batch operations)


The server will start on port 80 by default.

## Security

The application uses:
- Middleware for authentication via Solana wallet signatures
- Cookie-based sessions
- Proxy support for Ratelimit bypass

## Dependencies

- `github.com/andybalholm/brotli`: Brotli compression algorithm implementation
- `github.com/gagliardetto/solana-go`: Solana blockchain client for Go
- `github.com/go-sql-driver/mysql`: MySQL driver for Go
- `github.com/google/uuid`: UUID generation for operation tracking