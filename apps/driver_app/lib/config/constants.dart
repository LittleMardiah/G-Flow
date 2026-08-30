const String kAppName = 'G-Flow Driver';

/// Base URL backend. Sama dengan customer_app & merchant_app agar bisa
/// ngobrol dengan backend lokal yang sama.
const String kApiBaseUrl = 'http://10.184.247.135:8080';

/// Kapasitas maksimum order aktif per driver (ROADMAP 3.10: >3 -> tolak).
const int kMaxActiveOrders = 3;

/// Interval publish lokasi driver (detik) — ROADMAP 3.10 location_provider.
const int kLocationPublishIntervalSeconds = 5;

const double kDriverMinBalanceThreshold = 50000;

String formatRupiah(int amount) {
  final negative = amount < 0;
  var s = amount.abs().toString();
  final buf = StringBuffer();
  for (var i = 0; i < s.length; i++) {
    buf.write(s[i]);
    final remaining = s.length - 1 - i;
    if (remaining > 0 && remaining % 3 == 0) buf.write('.');
  }
  return '${negative ? '-' : ''}Rp $buf';
}

String shortId(String id) =>
    id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();