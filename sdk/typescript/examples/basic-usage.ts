import { GoatflowClient, isNotFoundError, isUnauthorizedError } from '../src';

// Example 1: Basic usage with API key
async function basicExample() {
  console.log('🚀 Basic GoatFlow SDK Example');

  // Initialize client with API key
  const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

  try {
    // Test connection
    const isConnected = await client.ping();
    if (!isConnected) {
      throw new Error('Failed to connect to GoatFlow API');
    }
    console.log('✅ Connected to GoatFlow successfully');

    // Get dashboard stats
    const stats = await client.dashboard.getStats();
    console.log('📊 Dashboard Stats:', {
      totalTickets: stats.total_tickets,
      openTickets: stats.open_tickets,
      myTickets: stats.my_tickets,
    });

    // List recent tickets
    const tickets = await client.tickets.list({
      page_size: 5,
      sort_by: 'created_at',
      sort_order: 'desc',
    });
    
    console.log(`📋 Found ${tickets.total_count} tickets (showing ${tickets.tickets.length})`);
    tickets.tickets.forEach(ticket => {
      console.log(`  - #${ticket.ticket_number}: ${ticket.title} (${ticket.status})`);
    });

    // Create a new ticket
    const newTicket = await client.tickets.create({
      title: 'SDK Test Ticket',
      description: 'This ticket was created using the TypeScript SDK',
      priority: 'normal',
      type: 'incident',
      queue_id: 1,
      customer_id: 1,
      tags: ['sdk', 'test'],
    });
    
    console.log(`✅ Created ticket #${newTicket.ticket_number} with ID ${newTicket.id}`);

    // Add a message to the ticket
    const message = await client.tickets.addMessage(newTicket.id, {
      content: 'This is a test message added via the TypeScript SDK',
      message_type: 'note',
      is_internal: false,
    });
    
    console.log(`💬 Added message with ID ${message.id}`);

    // Update ticket priority
    const updatedTicket = await client.tickets.update(newTicket.id, {
      priority: 'high',
    });
    
    console.log(`🔄 Updated ticket priority to ${updatedTicket.priority}`);

    // Search tickets
    const searchResults = await client.tickets.search('SDK', {
      page_size: 3,
    });
    
    console.log(`🔍 Found ${searchResults.total_count} tickets matching 'SDK'`);

    // Close the test ticket
    const closedTicket = await client.tickets.close(newTicket.id, 'Test completed');
    console.log(`🔒 Closed ticket #${closedTicket.ticket_number}`);

  } catch (error) {
    console.error('❌ Error:', error);
  }
}

// Example 2: Authentication flow
async function authenticationExample() {
  console.log('\n🔐 Authentication Example');

  // Create client without authentication first
  const client = new GoatflowClient({
    baseURL: 'https://your-goatflow-instance.com',
  });

  try {
    // Login with email and password
    await client.login('user@example.com', 'password');
    console.log('✅ Logged in successfully');

    // Now we can access protected resources
    const profile = await client.auth.getProfile();
    console.log(`👤 Logged in as: ${profile.first_name} ${profile.last_name} (${profile.email})`);

    // Get my tickets
    const myTickets = await client.dashboard.getMyTickets();
    console.log(`📝 I have ${myTickets.length} assigned tickets`);

    // Logout
    await client.logout();
    console.log('✅ Logged out successfully');

  } catch (error) {
    if (isUnauthorizedError(error)) {
      console.error('❌ Authentication failed: Invalid credentials');
    } else {
      console.error('❌ Error:', error);
    }
  }
}

// Example 3: Error handling
async function errorHandlingExample() {
  console.log('\n🚨 Error Handling Example');

  const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

  try {
    // Try to get a non-existent ticket
    await client.tickets.get(999999);
    
  } catch (error) {
    if (isNotFoundError(error)) {
      console.log('✅ Properly handled NotFound error');
    } else if (isUnauthorizedError(error)) {
      console.log('❌ Authentication error - check your API key');
    } else {
      console.error('❌ Unexpected error:', error);
    }
  }

  try {
    // Try to create an invalid ticket
    await client.tickets.create({
      title: '', // Invalid: empty title
      description: 'Test',
    });
    
  } catch (error) {
    console.log('✅ Properly handled validation error');
  }
}

// Example 4: Real-time events
async function realTimeExample() {
  console.log('\n⚡ Real-time Events Example');

  const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

  try {
    // Connect to real-time events
    await client.events.connect();
    console.log('✅ Connected to real-time events');

    // Listen for ticket events
    const ticketSubscription = client.events.onTicket((event) => {
      console.log(`🎫 Ticket event: ${event.type}`, {
        ticketNumber: event.data.ticket_number,
        title: event.data.title,
        status: event.data.status,
      });
    });

    // Listen for message events
    const messageSubscription = client.events.onMessage((event) => {
      console.log(`💬 Message event: ${event.type}`, {
        ticketId: event.data.ticket_id,
        content: event.data.content.substring(0, 50) + '...',
      });
    });

    // Listen for connection events
    const connectionSubscription = client.events.onConnection({
      connected: () => console.log('🔗 Connected to WebSocket'),
      disconnected: (event) => console.log('❌ Disconnected:', event.reason),
      error: (error) => console.error('❌ WebSocket error:', error),
    });

    // Simulate some activity (in real app, this would happen naturally)
    console.log('⏳ Listening for events for 30 seconds...');
    
    setTimeout(() => {
      console.log('🛑 Stopping event listeners');
      ticketSubscription.unsubscribe();
      messageSubscription.unsubscribe();
      connectionSubscription.unsubscribe();
      client.events.disconnect();
    }, 30000);

  } catch (error) {
    console.error('❌ Real-time events error:', error);
  }
}

// Example 5: Concurrent operations
async function concurrentExample() {
  console.log('\n🚀 Concurrent Operations Example');

  const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

  try {
    // Perform multiple operations concurrently
    const [stats, tickets, users, queues] = await Promise.all([
      client.dashboard.getStats(),
      client.tickets.list({ page_size: 5 }),
      client.users.list(),
      client.queues.list(),
    ]);

    console.log('✅ Fetched data concurrently:');
    console.log(`  - Dashboard stats: ${stats.total_tickets} total tickets`);
    console.log(`  - Recent tickets: ${tickets.tickets.length} tickets`);
    console.log(`  - Users: ${users.length} users`);
    console.log(`  - Queues: ${queues.length} queues`);

    // Create multiple tickets concurrently
    const ticketPromises = Array.from({ length: 3 }, (_, i) =>
      client.tickets.create({
        title: `Concurrent Ticket ${i + 1}`,
        description: `This is ticket ${i + 1} created concurrently`,
        priority: 'normal',
        queue_id: 1,
        customer_id: 1,
      })
    );

    const createdTickets = await Promise.all(ticketPromises);
    console.log(`✅ Created ${createdTickets.length} tickets concurrently`);

    createdTickets.forEach((ticket, index) => {
      console.log(`  - Ticket ${index + 1}: #${ticket.ticket_number}`);
    });

  } catch (error) {
    console.error('❌ Concurrent operations error:', error);
  }
}

// Example 6: File upload
async function fileUploadExample() {
  console.log('\n📎 File Upload Example');

  const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

  try {
    // First create a ticket
    const ticket = await client.tickets.create({
      title: 'Ticket with Attachment',
      description: 'This ticket will have a file attachment',
      queue_id: 1,
      customer_id: 1,
    });

    // Create a simple text file (in Node.js environment)
    if (typeof Buffer !== 'undefined') {
      const fileContent = 'This is a test file attachment from the TypeScript SDK';
      const fileBuffer = Buffer.from(fileContent, 'utf-8');

      const attachment = await client.tickets.uploadAttachment(
        ticket.id,
        fileBuffer,
        'test-file.txt'
      );

      console.log(`✅ Uploaded attachment: ${attachment.filename} (${attachment.size} bytes)`);

      // List all attachments for the ticket
      const attachments = await client.tickets.getAttachments(ticket.id);
      console.log(`📎 Ticket has ${attachments.length} attachment(s)`);

      // Download the attachment
      const downloadedData = await client.tickets.downloadAttachment(ticket.id, attachment.id);
      console.log(`⬇️ Downloaded ${downloadedData.byteLength} bytes`);
    } else {
      console.log('ℹ️ File upload example requires Node.js Buffer support');
    }

  } catch (error) {
    console.error('❌ File upload error:', error);
  }
}

// Run all examples
async function runAllExamples() {
  await basicExample();
  await authenticationExample();
  await errorHandlingExample();
  await realTimeExample();
  await concurrentExample();
  await fileUploadExample();
  
  console.log('\n✨ All examples completed!');
}

// Export examples for individual testing
export {
  basicExample,
  authenticationExample,
  errorHandlingExample,
  realTimeExample,
  concurrentExample,
  fileUploadExample,
  runAllExamples,
};

// Run examples if this file is executed directly
if (require.main === module) {
  runAllExamples().catch(console.error);
}