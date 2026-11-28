import test from 'node:test';
import assert from 'node:assert/strict';
import net from 'node:net';

function startMockSmtpServer() {
  const server = net.createServer((socket) => {
    socket.write('220 localhost ESMTP HanzoMail\r\n');
    socket.on('data', (chunk) => {
      const cmd = chunk.toString('utf8').trim();
      if (cmd.startsWith('EHLO')) socket.write('250-localhost\r\n250 OK\r\n');
      else if (cmd.startsWith('MAIL FROM:')) socket.write('250 OK\r\n');
      else if (cmd.startsWith('RCPT TO:')) socket.write('250 OK\r\n');
      else if (cmd === 'QUIT') {
        socket.write('221 Bye\r\n');
        socket.end();
      }
    });
  });
  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => resolve({ server, port: server.address().port }));
  });
}

function sendLine(socket, line) {
  return new Promise((resolve) => {
    socket.once('data', (d) => resolve(d.toString('utf8')));
    socket.write(`${line}\r\n`);
  });
}

test('loopback SMTP handshake completes EHLO -> MAIL FROM -> RCPT TO -> QUIT', async () => {
  const { server, port } = await startMockSmtpServer();
  const client = net.createConnection({ host: '127.0.0.1', port });
  try {
    const greeting = await new Promise((resolve) => client.once('data', (d) => resolve(d.toString('utf8'))));
    assert.match(greeting, /^220 /);
    assert.match(await sendLine(client, 'EHLO localhost'), /250 OK/);
    assert.match(await sendLine(client, 'MAIL FROM:<dev@hanzo.ai>'), /^250 OK/);
    assert.match(await sendLine(client, 'RCPT TO:<user@hanzo.ai>'), /^250 OK/);
    assert.match(await sendLine(client, 'QUIT'), /^221 /);
  } finally {
    client.destroy();
    await new Promise((resolve) => server.close(resolve));
  }
});
