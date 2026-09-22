import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/wallet.dart';
import '../../providers/wallet_provider.dart';

/// Filter `reference_type` riwayat wallet. Nilai = konstanta ref_type yang
/// ditulis backend (internal/wallet/service.go TOPUP/TRANSFER,
/// ride/food/send escrow-settle-refund, admin REVERSAL).
const List<String> _refTypeOptions = [
  'TOPUP',
  'TRANSFER',
  'RIDE_ESCROW',
  'RIDE_SETTLEMENT',
  'RIDE_REFUND',
  'FOOD_ESCROW',
  'FOOD_REFUND',
  'FOOD_SETTLEMENT',
  'SEND_ESCROW',
  'SEND_REFUND',
  'SEND_SETTLEMENT',
  'REVERSAL',
];

/// Riwayat wallet (TD-060, API_CONTRACT 6.5) — list LedgerEntry dengan
/// pagination (scroll listener, pola sama ride/food history) + dropdown
/// filter `reference_type` opsional. Argumen navigasi: `WalletHistoryArgs`.
/// Tap item saat ini no-op (belum ada screen detail ledger).
class WalletHistoryScreen extends ConsumerStatefulWidget {
  const WalletHistoryScreen({super.key});

  @override
  ConsumerState<WalletHistoryScreen> createState() =>
      _WalletHistoryScreenState();
}

class _WalletHistoryScreenState extends ConsumerState<WalletHistoryScreen> {
  final ScrollController _scroll = ScrollController();
  String _selectedFilter = 'ALL';
  String _walletId = '';
  bool _inited = false;

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scroll.removeListener(_onScroll);
    _scroll.dispose();
    super.dispose();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_inited) return;
    final raw = ModalRoute.of(context)?.settings.arguments;
    if (raw is WalletHistoryArgs) {
      _inited = true;
      _walletId = raw.walletId;
      _selectedFilter = raw.referenceType ?? 'ALL';
      final args = _args;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) ref.read(walletHistoryProvider(args).notifier).loadFirst();
      });
    }
  }

  WalletHistoryArgs get _args => WalletHistoryArgs(
        walletId: _walletId,
        referenceType: _selectedFilter == 'ALL' ? null : _selectedFilter,
      );

  void _onScroll() {
    if (!_scroll.hasClients) return;
    if (_scroll.position.pixels < _scroll.position.maxScrollExtent - 200) return;
    final args = _args;
    final state = ref.read(walletHistoryProvider(args));
    if (state.isLoading || !state.hasMore) return;
    ref.read(walletHistoryProvider(args).notifier).loadMore();
  }

  void _onFilterChanged(String value) {
    setState(() => _selectedFilter = value);
    final args = _args;
    ref.read(walletHistoryProvider(args).notifier).loadFirst();
  }

  @override
  Widget build(BuildContext context) {
    if (_walletId.isEmpty) return const _MissingArgsView();
    final state = ref.watch(walletHistoryProvider(_args));

    return Scaffold(
      appBar: AppBar(title: const Text('Riwayat Wallet')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: DropdownButtonFormField<String>(
              initialValue: _selectedFilter,
              decoration: const InputDecoration(
                labelText: 'Filter Tipe Transaksi',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              items: [
                const DropdownMenuItem(
                  value: 'ALL',
                  child: Text('Semua'),
                ),
                for (final t in _refTypeOptions)
                  DropdownMenuItem(value: t, child: Text(t)),
              ],
              onChanged: (value) {
                if (value != null) _onFilterChanged(value);
              },
            ),
          ),
          Expanded(child: _buildBody(context, state)),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context, WalletHistoryState state) {
    if (state.isLoading && state.entries.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.entries.isEmpty) {
      return _ErrorView(
        message: state.error!,
        onRetry: () => ref.read(walletHistoryProvider(_args).notifier).loadFirst(),
      );
    }
    if (state.entries.isEmpty) {
      return const Center(child: Text('Belum ada transaksi.'));
    }
    return ListView.builder(
      controller: _scroll,
      padding: const EdgeInsets.all(16),
      itemCount: state.entries.length + (state.hasMore ? 1 : 0),
      itemBuilder: (context, i) {
        if (i >= state.entries.length) {
          return const Padding(
            padding: EdgeInsets.all(16),
            child: Center(child: CircularProgressIndicator()),
          );
        }
        return _LedgerTile(entry: state.entries[i]);
      },
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
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(credit ? Icons.south_west : Icons.north_east, size: 18, color: color),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    entry.description.isNotEmpty
                        ? entry.description
                        : entry.referenceType.isNotEmpty
                            ? entry.referenceType
                            : entry.entryType,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodyMedium,
                  ),
                ),
                Text(
                  '${credit ? '+' : '-'}${_formatRupiah(entry.amount)}',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: color,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Row(
              children: [
                if (entry.referenceType.isNotEmpty)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.primary.withValues(alpha: 0.10),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      entry.referenceType,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.primary,
                        fontSize: 11,
                      ),
                    ),
                  ),
                const Spacer(),
                if (entry.createdAt != null)
                  Text(
                    _formatDate(entry.createdAt!),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 2),
            Text(
              'Saldo setelah: ${_formatRupiah(entry.balanceAfter)}',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
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

class _MissingArgsView extends StatelessWidget {
  const _MissingArgsView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Riwayat Wallet')),
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

String _formatDate(DateTime dt) {
  final local = dt.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}/${two(local.month)}/${local.year} '
      '${two(local.hour)}:${two(local.minute)}';
}