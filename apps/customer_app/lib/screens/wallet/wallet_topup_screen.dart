import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/wallet_provider.dart';
import 'wallet_balance_screen.dart';

/// Top-Up wallet (TD-060, API_CONTRACT 6.2) — input nominal + tombol cepat
/// (50K/100K/500K/1M). Submit memanggil `topUp()` lalu menutup screen;
/// saldo diperbarui via `load()` di notifier (balance screen auto-refresh
/// karena meng-watch provider yang sama).
class WalletTopupScreen extends ConsumerStatefulWidget {
  const WalletTopupScreen({super.key});

  @override
  ConsumerState<WalletTopupScreen> createState() => _WalletTopupScreenState();
}

class _WalletTopupScreenState extends ConsumerState<WalletTopupScreen> {
  final TextEditingController _amountController = TextEditingController();
  bool _submitting = false;

  static const _quickAmounts = [50000, 100000, 500000, 1000000];

  @override
  void dispose() {
    _amountController.dispose();
    super.dispose();
  }

  void _setAmount(int amount) {
    _amountController.text = amount.toString();
  }

  Future<void> _submit(String walletId) async {
    if (_submitting) return;
    final raw = _amountController.text.trim();
    final amount = int.tryParse(raw);
    if (amount == null || amount <= 0) {
      _showSnack('Nominal top-up harus berupa angka lebih dari 0.');
      return;
    }

    setState(() => _submitting = true);
    final ok = await ref
        .read(walletBalanceProvider(walletId).notifier)
        .topUp(amount);
    if (!mounted) return;
    setState(() => _submitting = false);

    if (ok) {
      // Balance screen memakai myWalletProvider (auto-resolve /wallets/me);
      // refresh di-fire-and-forget, screen rebuild saat load selesai.
      ref.read(myWalletProvider.notifier).load();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Top-up berhasil.')),
      );
      Navigator.of(context).pop(walletId);
    } else {
      _showSnack('Gagal melakukan top-up. Periksa koneksi lalu coba lagi.');
    }
  }

  void _showSnack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final raw = ModalRoute.of(context)?.settings.arguments;
    if (raw is! WalletArgs) return const _MissingArgsView();

    final walletId = raw.walletId;

    return Scaffold(
      appBar: AppBar(title: const Text('Top-Up Wallet')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            controller: _amountController,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            decoration: const InputDecoration(
              labelText: 'Nominal',
              prefixText: 'Rp ',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final amount in _quickAmounts)
                ActionChip(
                  label: Text(_formatRupiah(amount)),
                  onPressed: () => _setAmount(amount),
                ),
            ],
          ),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: _submitting ? null : () => _submit(walletId),
            child: Text(_submitting ? 'Memproses…' : 'Top-Up Sekarang'),
          ),
        ],
      ),
    );
  }
}

class _MissingArgsView extends StatelessWidget {
  const _MissingArgsView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Top-Up Wallet')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            const Text('Argumen wallet tidak ditemukan.'),
            const SizedBox(height: 8),
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Kembali'),
            ),
          ],
        ),
      ),
    );
  }
}

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}