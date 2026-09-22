import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/wallet_provider.dart';
import 'wallet_balance_screen.dart';

/// Transfer P2P wallet (TD-060, API_CONTRACT 6.3) — input wallet tujuan,
/// nominal, dan deskripsi opsional. Submit memanggil `transfer()` lalu
/// menutup screen; saldo diperbarui via `load()` di notifier.
class WalletTransferScreen extends ConsumerStatefulWidget {
  const WalletTransferScreen({super.key});

  @override
  ConsumerState<WalletTransferScreen> createState() =>
      _WalletTransferScreenState();
}

class _WalletTransferScreenState extends ConsumerState<WalletTransferScreen> {
  final TextEditingController _toController = TextEditingController();
  final TextEditingController _amountController = TextEditingController();
  final TextEditingController _descriptionController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _toController.dispose();
    _amountController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  Future<void> _submit(String walletId) async {
    if (_submitting) return;
    final toWalletId = _toController.text.trim();
    final amount = int.tryParse(_amountController.text.trim());
    if (toWalletId.isEmpty) {
      _showSnack('Wallet tujuan wajib diisi.');
      return;
    }
    if (amount == null || amount <= 0) {
      _showSnack('Nominal transfer harus berupa angka lebih dari 0.');
      return;
    }

    setState(() => _submitting = true);
    final ok = await ref
        .read(walletBalanceProvider(walletId).notifier)
        .transfer(
          toWalletId: toWalletId,
          amount: amount,
          description: _descriptionController.text.trim(),
        );
    if (!mounted) return;
    setState(() => _submitting = false);

    if (ok) {
      // Balance screen memakai myWalletProvider (auto-resolve /wallets/me);
      // refresh di-fire-and-forget, screen rebuild saat load selesai.
      ref.read(myWalletProvider.notifier).load();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Transfer berhasil.')),
      );
      Navigator.of(context).pop(walletId);
    } else {
      _showSnack('Gagal melakukan transfer. Periksa koneksi lalu coba lagi.');
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
      appBar: AppBar(title: const Text('Transfer')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            controller: _toController,
            decoration: const InputDecoration(
              labelText: 'Wallet Tujuan',
              hintText: 'ID wallet penerima',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
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
          TextField(
            controller: _descriptionController,
            decoration: const InputDecoration(
              labelText: 'Deskripsi (opsional)',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: _submitting ? null : () => _submit(walletId),
            child: Text(_submitting ? 'Memproses…' : 'Transfer Sekarang'),
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
      appBar: AppBar(title: const Text('Transfer')),
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