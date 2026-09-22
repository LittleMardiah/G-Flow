import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../config/router.dart';
import '../../models/wallet.dart';
import '../../providers/wallet_provider.dart';

/// Argumen navigasi layar top-up / transfer / riwayat wallet. Balance screen
/// tidak lagi memakai argumen ini: wallet_id di-resolve via GET /wallets/me
/// (TD-120).
class WalletArgs {
  const WalletArgs({required this.walletId});

  final String walletId;
}

/// Wallet Balance (TD-060, API_CONTRACT 6.1) — kartu saldo besar + tombol
/// Top-Up / Transfer / Riwayat + ringkasan 5 transaksi terbaru.
///
/// Saldo auto-resolve via `myWalletProvider` (GET /wallets/me) sehingga
/// screen tidak butuh WalletArgs. Riwayat terbaru dimuat sekali setelah
/// wallet ter-resolve (`walletHistoryProvider`).
class WalletBalanceScreen extends ConsumerStatefulWidget {
  const WalletBalanceScreen({super.key});

  @override
  ConsumerState<WalletBalanceScreen> createState() =>
      _WalletBalanceScreenState();
}

class _WalletBalanceScreenState extends ConsumerState<WalletBalanceScreen> {
  WalletHistoryArgs? _historyArgs;
  String? _historyLoadedWalletId;

  void _ensureHistory(Wallet wallet) {
    final walletId = wallet.walletId;
    if (walletId.isEmpty || walletId == _historyLoadedWalletId) return;
    _historyLoadedWalletId = walletId;
    _historyArgs = WalletHistoryArgs(walletId: walletId);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final current = ref.read(myWalletProvider).wallet;
      if (current == null || current.walletId != walletId) return;
      ref
          .read(walletHistoryProvider(_historyArgs!).notifier)
          .loadFirst();
    });
  }

  @override
  Widget build(BuildContext context) {
    final balance = ref.watch(myWalletProvider);
    final wallet = balance.wallet;

    if (wallet == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Wallet')),
        body: balance.isLoading || balance.error == null
            ? const Center(child: CircularProgressIndicator())
            : _ErrorView(
                message: balance.error!,
                onRetry: () => ref.read(myWalletProvider.notifier).load(),
              ),
      );
    }

    final walletId = wallet.walletId;
    if (walletId.isEmpty) {
      return Scaffold(
        appBar: AppBar(title: const Text('Wallet')),
        body: _ErrorView(
          message: 'Server tidak mengembalikan wallet_id. Mulai ulang aplikasi.',
          onRetry: () => ref.read(myWalletProvider.notifier).load(),
        ),
      );
    }

    _ensureHistory(wallet);
    final history = ref.watch(walletHistoryProvider(_historyArgs!));

    return Scaffold(
      appBar: AppBar(title: const Text('Wallet')),
      body: RefreshIndicator(
        onRefresh: () async {
          await ref.read(myWalletProvider.notifier).load();
          if (_historyArgs != null) {
            await ref
                .read(walletHistoryProvider(_historyArgs!).notifier)
                .loadFirst();
          }
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _BalanceCard(walletId: walletId, balance: wallet.balance),
            const SizedBox(height: 16),
            _ActionRow(walletId: walletId),
            const SizedBox(height: 24),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('Transaksi Terbaru', style: Theme.of(context).textTheme.titleMedium),
                TextButton(
                  onPressed: () => Navigator.of(context).pushNamed(
                    AppRoutes.walletHistory,
                    arguments: WalletHistoryArgs(walletId: walletId),
                  ),
                  child: const Text('Lihat Semua'),
                ),
              ],
            ),
            const SizedBox(height: 4),
            if (history.isLoading && history.entries.isEmpty)
              const Padding(
                padding: EdgeInsets.all(16),
                child: Center(child: CircularProgressIndicator()),
              )
            else if (history.entries.isEmpty)
              const Padding(
                padding: EdgeInsets.all(16),
                child: Center(child: Text('Belum ada transaksi.')),
              )
            else
              ...history.entries.take(5).map((e) => _LedgerTile(entry: e)),
          ],
        ),
      ),
    );
  }
}

class _BalanceCard extends StatelessWidget {
  const _BalanceCard({required this.walletId, required this.balance});

  final String walletId;
  final int balance;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            theme.colorScheme.primary,
            theme.colorScheme.primary.withValues(alpha: 0.7),
          ],
        ),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Saldo',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onPrimary.withValues(alpha: 0.9),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            _formatRupiah(balance),
            style: theme.textTheme.headlineMedium?.copyWith(
              color: theme.colorScheme.onPrimary,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Wallet ID: ${_shortId(walletId)}',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onPrimary.withValues(alpha: 0.9),
            ),
          ),
        ],
      ),
    );
  }
}

class _ActionRow extends StatelessWidget {
  const _ActionRow({required this.walletId});

  final String walletId;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: FilledButton.icon(
            onPressed: () => Navigator.of(context).pushNamed(
              AppRoutes.walletTopup,
              arguments: WalletArgs(walletId: walletId),
            ),
            icon: const Icon(Icons.add),
            label: const Text('Top-Up'),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: OutlinedButton.icon(
            onPressed: () => Navigator.of(context).pushNamed(
              AppRoutes.walletTransfer,
              arguments: WalletArgs(walletId: walletId),
            ),
            icon: const Icon(Icons.swap_horiz),
            label: const Text('Transfer'),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: OutlinedButton.icon(
            onPressed: () => Navigator.of(context).pushNamed(
              AppRoutes.walletHistory,
              arguments: WalletHistoryArgs(walletId: walletId),
            ),
            icon: const Icon(Icons.history),
            label: const Text('Riwayat'),
          ),
        ),
      ],
    );
  }
}

class _LedgerTile extends StatelessWidget {
  const _LedgerTile({required this.entry});

  final LedgerEntry entry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final credit = !entry.entryType.toUpperCase().startsWith('DEBIT');
    final color = credit ? Colors.green.shade600 : theme.colorScheme.error;
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(credit ? Icons.south_west : Icons.north_east, size: 18, color: color),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    entry.description.isNotEmpty
                        ? entry.description
                        : entry.referenceType.isNotEmpty
                            ? entry.referenceType
                            : entry.entryType,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodyMedium,
                  ),
                  if (entry.createdAt != null)
                    Text(
                      _formatDate(entry.createdAt!),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '${credit ? '+' : '-'}${_formatRupiah(entry.amount)}',
              style: theme.textTheme.titleSmall?.copyWith(
                color: color,
                fontWeight: FontWeight.bold,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off, size: 48, color: Colors.grey),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh),
            label: const Text('Coba lagi'),
          ),
        ],
      ),
    );
  }
}

String _shortId(String id) =>
    id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}

String _formatDate(DateTime dt) {
  final local = dt.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}/${two(local.month)}/${local.year} '
      '${two(local.hour)}:${two(local.minute)}';
}