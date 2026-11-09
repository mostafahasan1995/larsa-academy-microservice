// // MongoDB initialization script
// // This script runs when the MongoDB container starts for the first time

// db = db.getSiblingDB('larsa_academy');

// // Create initial collections with basic indexes
// db.createCollection('users');
// db.createCollection('courses');
// db.createCollection('enrollments');
// db.createCollection('notifications');
// db.createCollection('analytics');
// db.createCollection('academyBundles');

// // Create indexes for better performance
// db.users.createIndex({ "email": 1 }, { unique: true });
// db.users.createIndex({ "created_at": 1 });
// db.users.createIndex({ "status": 1 });

// db.courses.createIndex({ "title": "text", "description": "text" });
// db.courses.createIndex({ "category": 1, "status": 1 });
// db.courses.createIndex({ "created_at": 1 });

// db.enrollments.createIndex({ "user_id": 1, "course_id": 1 }, { unique: true });
// db.enrollments.createIndex({ "user_id": 1 });
// db.enrollments.createIndex({ "course_id": 1 });
// db.enrollments.createIndex({ "enrollment_date": 1 });

// db.notifications.createIndex({ "user_id": 1, "read": 1 });
// db.notifications.createIndex({ "created_at": 1 });
// db.notifications.createIndex({ "type": 1 });

// db.analytics.createIndex({ "event_type": 1, "timestamp": 1 });
// db.analytics.createIndex({ "user_id": 1, "timestamp": 1 });

// // Indexes for academyBundles collection
// db.academyBundles.createIndex({ "trash": 1, "_id": -1 });  // Main compound index for listing
// db.academyBundles.createIndex({ "status": 1, "trash": 1 }); // For status-based queries
// db.academyBundles.createIndex({ "country": 1, "trash": 1 }); // For country-based queries
// db.academyBundles.createIndex({ "schoolType": 1, "trash": 1 }); // For school type queries
// db.academyBundles.createIndex({ "subject": 1, "trash": 1 }); // For subject-based queries
// db.academyBundles.createIndex({ "createdAt": -1 }); // For time-based queries
// db.academyBundles.createIndex({ "content.type": 1, "trash": 1 }); // For content type queries

// print('Database initialization completed successfully');
