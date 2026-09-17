import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../providers/auth_provider.dart';

class LoginScreen extends ConsumerWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    final emailController = TextEditingController();
    final passController = TextEditingController();

    return Scaffold(
      appBar: AppBar(title: const Text('Login')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: SingleChildScrollView(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              TextField(
                controller: emailController,
                decoration: const InputDecoration(labelText: 'Email'),
              ),
              const SizedBox(height: 16),
              TextField(
                controller: passController,
                obscureText: true,
                decoration: const InputDecoration(labelText: 'Password'),
              ),
              const SizedBox(height: 24),
              if (auth.isLoading) const CircularProgressIndicator(),
              if (auth.error != null) Text(auth.error!, style: const TextStyle(color: Colors.red)),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: auth.isLoading ? null : () async {
                  if (emailController.text.trim().isEmpty || passController.text.isEmpty) {
                    ref.read(authProvider.notifier).state = ref.read(authProvider.notifier).state.copyWith(
                      error: 'Email dan password wajib diisi'
                    );
                    return;
                  }
                  await ref.read(authProvider.notifier).login(
                    emailController.text.trim(),
                    passController.text,
                  );
                  if (ref.read(authProvider).isAuthenticated) {
                    Navigator.pushReplacementNamed(context, '/home');
                  }
                },
                child: const Text('Login'),
              ),
              const SizedBox(height: 8),
              TextButton(
                onPressed: () => Navigator.pushNamed(context, '/register'),
                child: const Text('Register'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
