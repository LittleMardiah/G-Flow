import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../providers/auth_provider.dart';

class RegisterScreen extends ConsumerWidget {
  const RegisterScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    final nameController = TextEditingController();
    final emailController = TextEditingController();
    final passController = TextEditingController();
    final phoneController = TextEditingController();

    return Scaffold(
      appBar: AppBar(title: const Text('Register')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            TextField(controller: nameController, decoration: const InputDecoration(labelText: 'Name')),
            TextField(controller: emailController, decoration: const InputDecoration(labelText: 'Email')),
            TextField(controller: phoneController, decoration: const InputDecoration(labelText: 'Phone')),
            TextField(controller: passController, obscureText: true, decoration: const InputDecoration(labelText: 'Password')),
            const SizedBox(height: 20),
            if (auth.isLoading) const CircularProgressIndicator(),
            if (auth.error != null) Text(auth.error!, style: const TextStyle(color: Colors.red)),
            ElevatedButton(
              onPressed: auth.isLoading ? null : () async {
                await ref.read(authProvider.notifier).register({
                  'name': nameController.text,
                  'email': emailController.text,
                  'phone': phoneController.text,
                  'password': passController.text,
                  'user_type': 'customer',
                });
                if (!context.mounted) return;
                if (auth.error == null) {
                  Navigator.pop(context);
                }
              },
              child: const Text('Register'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Already have account? Login'),
            ),
          ],
        ),
      ),
    );
  }
}
