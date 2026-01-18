Uber System Design Document
This document outlines the design for a real-time ride-sharing system similar to Uber, focusing on core functionality, scalability, and consistency during the matching process.
1. Requirements
1.1 Functional Requirements
The system focuses on these core "user should be able to" statements:
• Request Fair Estimate: Users can input a start location and destination to receive an estimated fair and ETA.
• Request a Ride: Users can request a ride based on an estimate to be matched with a nearby available driver in real-time.
• Accept/Deny Requests: Drivers can accept or deny ride requests sent to them.
• Navigate: Drivers can navigate to the user's pickup location and subsequently to the destination.
• Location Updates: Drivers must be able to provide periodic location updates.
Out of Scope: Multiple car types (UberX only), driver/rider ratings, scheduled rides, and complex payment processing.
1.2 Non-Functional Requirements
The system must prioritize the following qualities:
• Low Latency Matching: The system should match a rider to a driver in less than one minute.
• Consistency: Matching must be one-to-one; a single ride should only be matched to one driver, and a single driver should only receive one request at a time.
• High Availability: The system should be highly available outside of the specific matching process to ensure users can always access the app.
• High Throughput: The architecture must handle massive surges during peak hours or special events (e.g., hundreds of thousands of requests in a single region).

--------------------------------------------------------------------------------
2. Core Entities
The following objects are persisted and exchanged within the system:
• Rider: Contains ID and metadata (e.g., payment info).
• Driver: Contains ID, metadata (car, license plate), and status (Available, In Ride, Offline).
• Ride: Contains Ride ID, Rider ID, Driver ID (optional until matched), Fair, ETA, Source, Destination, and Status.
• Location: Stores the most up-to-date latitude and longitude for all active drivers to facilitate proximity-based matching.

--------------------------------------------------------------------------------
3. API Design
Endpoint
Method
Input
Output
Description
/ride/fair-estimate
POST
source, destination
rideID, ETA, price
Returns a partial ride entity with pricing.
/ride/request
PATCH
rideID
200 OK / 400 Error
Asynchronously starts the matching process.
/location/update
PATCH
lat, long
200 OK
Called by driver clients every ~5 seconds.
/ride/driver/accept
PATCH
rideID, accept (bool)
200 OK
Driver accepts or denies a specific ride.
/ride/driver/update
PATCH
rideID, status
next_lat_long / null
Updates ride status (e.g., "Picked Up") and returns next destination.
Note: Security is handled via JWT or session tokens in the request header rather than passing userID in the body.

--------------------------------------------------------------------------------
4. High-Level Architecture
4.1 System Components
1. API Gateway: An AWS-managed gateway handles load balancing, routing to microservices, authentication, and rate limiting.
2. Ride Service: Manages fair estimates and ride persistence. It uses third-party mapping services (like Google Maps) to calculate ETAs and prices.
3. Ride Matching Service: A computationally expensive, asynchronous service that identifies eligible drivers within a radius and manages the matching loop.
4. Location Service: Updates and queries the Location DB to track driver movement.
5. Notification Service: Uses push notifications (APNS/Firebase) to alert drivers of new requests.
6. Primary Database: A horizontally scalable database (like DynamoDB) to store Rider, Driver, and Ride entities.
4.2 The Matching Flow
1. Request: The rider requests a ride; the request is placed in a Ride Request Queue (partitioned by region) to handle surges and ensure resilience.
2. Discovery: The Matching Service queries the Location DB for drivers within N miles.
3. Filtering: Drivers are filtered based on an "Available" status in the primary DB.
4. Notification: The service iterates through the driver list, sending push notifications one by one until the ride is accepted.

--------------------------------------------------------------------------------
5. Key Design Decisions (Deep Dives)
5.1 Real-Time Location Tracking (Redis + Geohashing)
To handle approximately 600,000 updates per second (assuming 3M active drivers updating every 5 seconds), the system uses Redis with Geohashing.
• Why Geohashing? Unlike Quad Trees, geohashing does not require complex re-indexing on every update. It treats locations as strings, making it efficient for high-frequency writes while still supporting proximity searches.
• Optimization: Clients can use Dynamic Location Updates, sending updates less frequently if the driver is stationary or in a low-demand area.
5.2 Consistency and Distributed Locking
To prevent "double matching" (multiple drivers getting the same ride or one driver getting multiple rides), the system employs Distributed Locks.
• Mechanism: When a matching request is sent to a driver, a lock is placed on that driverID in the database (or Redis) with a TTL (Time-to-Live) of ~5-10 seconds.
• Benefit: If the driver does not respond, the lock automatically expires, making the driver available for other requests without requiring a manual "unlock" or a slow cron job.
