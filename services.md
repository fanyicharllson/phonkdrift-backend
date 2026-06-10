Service Name,Primary Protocol,Tech Stack,Responsibility
0. API Gateway / Proxy,HTTP / gRPC Web / WebSockets,Envoy Proxy or custom Go Gateway,"The single entry point. Routing regular traffic, managing WebSocket handshakes, and enforcing security."
1. Auth & User Service,gRPC,Go + PostgreSQL (Supabase) + JWT,"User accounts, profiles, relationships (followers), and basic data storage."
2. Track & Stream Service,gRPC,Go + yt-dlp + Redis (Caching) + Gobreaker,"Handles music searches, fetching YouTube streams, caching track links, and recording global trend statistics."
3. Community & Chat Service,WebSockets & gRPC,Go + Redis Pub/Sub + MongoDB or PostgreSQL,"Real-time global/room chat, sharing track items natively into the chat stream, and message persistence."
4. Event Notification Service,RabbitMQ consumer,Go + Firebase Cloud Messaging (FCM) API,Listening to the message broker to blast out background push notifications for trends or direct message alerts.